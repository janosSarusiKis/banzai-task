/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"regexp"

	isd "github.com/jbenet/go-is-domain"

	"github.com/go-logr/logr"
	cmacme "github.com/jetstack/cert-manager/pkg/apis/acme/v1alpha3"
	v1alpha3 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DomainLabel = "domain"
	EmailLabel  = "email"
)

// CustomIngressManagerReconciler reconciles a CustomIngressManager object
type CustomIngressManagerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webapp.feladat.banzaicloud.io,resources=customingressmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.feladat.banzaicloud.io,resources=customingressmanagers/status,verbs=get;update;patch

func (r *CustomIngressManagerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("customingressmanager", req.NamespacedName)

	var service corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &service); err != nil {
		log.Error(err, "Unable to fetch the Service")

		var existingIngressPointer, innerIngressError = GetIngressAddressByServiceName(r, req.NamespacedName.Name+"-ingress", log)
		var existingClusterIssuerPointer, innerClusterIssuerError = GetIngressAddressByServiceName(r, req.NamespacedName.Name+"-lets-encrypt-staging", log)

		if innerIngressError != nil {
			return ctrl.Result{}, innerIngressError
		}

		if innerClusterIssuerError != nil {
			return ctrl.Result{}, innerIngressError
		}

		if existingIngressPointer != nil {
			r.Delete(ctx, existingIngressPointer)
		}

		if existingClusterIssuerPointer != nil {
			r.Delete(ctx, existingClusterIssuerPointer)
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if IsValidService(&service, log) {
		log.Info("Check if ingress already exists")

		var existingIngressPointer, innerIngressError = GetIngressAddressByServiceName(r, service.Name+"-ingress", log)
		var existingClusterIssuerPointer, innerClusterIssuerError = GetIngressAddressByServiceName(r, service.Name+"-lets-encrypt-staging", log)

		if innerIngressError != nil {
			return ctrl.Result{}, innerIngressError
		}

		if innerClusterIssuerError != nil {
			return ctrl.Result{}, innerClusterIssuerError
		}

		if existingClusterIssuerPointer == nil {
			var clusterIssuer = v1alpha3.ClusterIssuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      service.Name + "-lets-encrypt-staging",
					Namespace: service.Namespace,
				},
				Spec: v1alpha3.IssuerSpec{
					IssuerConfig: v1alpha3.IssuerConfig{
						ACME: &cmacme.ACMEIssuer{
							Server: "https://acme-staging.api.letsencrypt.org/directory",
							Email:  service.ObjectMeta.Annotations[EmailLabel],
						},
					},
				},
			}
			log.Info("Try to create ClusterIssuer")
			if err := r.Create(ctx, &clusterIssuer); err != nil {
				log.Error(err, "unable to create the Cluster issuer")
				// we'll ignore not-found errors, since they can't be fixed by an immediate
				// requeue (we'll need to wait for a new notification), and we can get them
				// on deleted requests.
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}

		if existingIngressPointer == nil {
			var ingress = v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        service.Name + "-ingress",
					Namespace:   service.Namespace,
					Annotations: map[string]string{"cert-manager.io/cluster-issuer": service.Name + "-lets-encrypt-staging"},
				},
				Spec: v1beta1.IngressSpec{
					TLS: []v1beta1.IngressTLS{
						{
							Hosts:      []string{service.ObjectMeta.Annotations[DomainLabel]},
							SecretName: service.Name + "-secret",
						},
					},
					Rules: []v1beta1.IngressRule{
						{
							Host: service.ObjectMeta.Annotations[DomainLabel],
							IngressRuleValue: v1beta1.IngressRuleValue{
								HTTP: &v1beta1.HTTPIngressRuleValue{
									Paths: []v1beta1.HTTPIngressPath{
										{
											Path: "/",
											Backend: v1beta1.IngressBackend{
												ServiceName: service.Name,
												ServicePort: intstr.FromInt(80),
											},
										},
									},
								},
							},
						},
					},
				},
			}

			log.Info("Try to create Ingress")
			if err := r.Create(ctx, &ingress); err != nil {
				log.Error(err, "Unable to create the Ingress")
				// we'll ignore not-found errors, since they can't be fixed by an immediate
				// requeue (we'll need to wait for a new notification), and we can get them
				// on deleted requests.
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			log.Info("Ingress created")
		}
	}

	return ctrl.Result{}, nil
}

func (r *CustomIngressManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}

func GetIngressAddressByServiceName(r *CustomIngressManagerReconciler, ingressName string, log logr.Logger) (*v1beta1.Ingress, error) {
	ctx := context.Background()
	var currentIngresses v1beta1.IngressList
	if err := r.List(ctx, &currentIngresses); err != nil {
		return nil, err
	}

	for i := range currentIngresses.Items {
		if currentIngresses.Items[i].ObjectMeta.Name == ingressName {
			log.Info("Ingress already there")
			var ingress v1beta1.Ingress = currentIngresses.Items[i]

			return &ingress, nil
		}
	}

	return nil, nil
}

func GetClusterIssuerAddressByServiceName(r *CustomIngressManagerReconciler, clusterIssuerName string, log logr.Logger) (*v1alpha3.ClusterIssuer, error) {
	ctx := context.Background()
	var currentClusterIssuers v1alpha3.ClusterIssuerList
	if err := r.List(ctx, &currentClusterIssuers); err != nil {
		return nil, err
	}

	for i := range currentClusterIssuers.Items {
		if currentClusterIssuers.Items[i].ObjectMeta.Name == clusterIssuerName+"-ingress" {
			log.Info("ClusterIssuer already there")
			var clusterIssuer v1alpha3.ClusterIssuer = currentClusterIssuers.Items[i]

			return &clusterIssuer, nil
		}
	}

	return nil, nil
}

func IsValidService(service *corev1.Service, log logr.Logger) bool {
	const (
		customIngressLabel      = "feladat.banzaicloud.io/ingress"
		customIngressLabelValue = "secure"
	)

	regExValidaton := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	log.Info("Validating service")
	if customIngressLabelValue, result := service.ObjectMeta.Labels[customIngressLabel]; !result || customIngressLabelValue != customIngressLabelValue {
		log.Info("No custom label")

		return false
	}

	if domainLabelValue, result := service.ObjectMeta.Annotations[DomainLabel]; !result || !isd.IsDomain(domainLabelValue) {
		log.Info("Invalid domain name: " + domainLabelValue)

		return false
	}

	if emailLabelValue, result := service.ObjectMeta.Annotations[EmailLabel]; !result || !regExValidaton.MatchString(emailLabelValue) {
		log.Info("Invalid email address: " + emailLabelValue)

		return false
	}

	log.Info("Valid service found")

	return true
}
