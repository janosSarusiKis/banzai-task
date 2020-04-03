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
	"github.com/prometheus/common/log"

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
	DomainLabel             = "domain"
	EmailLabel              = "email"
	CustomIngressLabel      = "feladat.banzaicloud.io/ingress"
	CustomIngressLabelValue = "secure"
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

		existingIngress, err := r.GetIngressByName(CreateIngressName(req.NamespacedName.Name))
		existingClusterIssuer, err := r.GetClusterIssuerByName(CreateClusterIssuerName(req.NamespacedName.Name))

		if existingIngress != nil {
			r.Delete(ctx, existingIngress)
		}

		if existingClusterIssuer != nil {
			r.Delete(ctx, existingClusterIssuer)
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if r.IsValidService(&service) {
		log.Info("Check if ingress already exists")
		existingIngress, innerIngressError := r.GetIngressByName(CreateIngressName(service.Name))
		existingClusterIssuer, innerClusterIssuerError := r.GetClusterIssuerByName(CreateClusterIssuerName(service.Name))

		if innerIngressError != nil {
			return ctrl.Result{}, innerIngressError
		}

		if innerClusterIssuerError != nil {
			return ctrl.Result{}, innerClusterIssuerError
		}

		if existingClusterIssuer == nil {
			if err := r.CreateClusterIssuerForSerive(service); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}

		if existingIngress == nil {
			if err := r.CreateIngressForService(service); err != nil {
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *CustomIngressManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}

func (r *CustomIngressManagerReconciler) GetIngressByName(ingressName string) (*v1beta1.Ingress, error) {
	ctx := context.Background()
	var currentIngresses v1beta1.IngressList
	if err := r.List(ctx, &currentIngresses); err != nil {
		return nil, err
	}

	for _, ingress := range currentIngresses.Items {
		if ingress.ObjectMeta.Name == ingressName {
			r.Log.Info("Ingress already there")

			return &ingress, nil
		}
	}

	return nil, nil
}

func (r *CustomIngressManagerReconciler) GetClusterIssuerByName(clusterIssuerName string) (*v1alpha3.ClusterIssuer, error) {
	ctx := context.Background()
	var currentClusterIssuers v1alpha3.ClusterIssuerList
	if err := r.List(ctx, &currentClusterIssuers); err != nil {
		return nil, err
	}

	for _, clusterIssuer := range currentClusterIssuers.Items {
		if clusterIssuer.ObjectMeta.Name == clusterIssuerName {
			r.Log.Info("ClusterIssuer already there")

			return &clusterIssuer, nil
		}
	}

	return nil, nil
}

func (r *CustomIngressManagerReconciler) IsValidService(service *corev1.Service) bool {
	regExValidaton := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	r.Log.Info("Validating service")
	if customIngressLabelValue, ok := service.ObjectMeta.Labels[CustomIngressLabel]; !ok || customIngressLabelValue != customIngressLabelValue {
		r.Log.Info("No custom label")

		return false
	}

	if domainLabelValue, ok := service.ObjectMeta.Annotations[DomainLabel]; !ok || !isd.IsDomain(domainLabelValue) {
		r.Log.Info("Invalid domain name: " + domainLabelValue)

		return false
	}

	if emailLabelValue, ok := service.ObjectMeta.Annotations[EmailLabel]; !ok || !regExValidaton.MatchString(emailLabelValue) {
		r.Log.Info("Invalid email address: " + emailLabelValue)

		return false
	}

	r.Log.Info("Valid service found")

	return true
}

func (r *CustomIngressManagerReconciler) CreateIngressForService(service corev1.Service) error {
	ctx := context.Background()
	ingress := v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        CreateIngressName(service.Name),
			Namespace:   service.Namespace,
			Annotations: map[string]string{"cert-manager.io/cluster-issuer": CreateClusterIssuerName(service.Name)},
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
		return client.IgnoreNotFound(err)
	}

	log.Info("Ingress created")

	return nil
}

func (r *CustomIngressManagerReconciler) CreateClusterIssuerForSerive(service corev1.Service) error {
	ctx := context.Background()
	clusterIssuer := v1alpha3.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CreateClusterIssuerName(service.Name),
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

	r.Log.Info("Try to create ClusterIssuer")
	if err := r.Create(ctx, &clusterIssuer); err != nil {
		log.Error(err, "Unable to create the Cluster issuer")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return client.IgnoreNotFound(err)
	}

	return nil
}

func CreateIngressName(name string) string {
	return name + "-ingeress"
}

func CreateClusterIssuerName(name string) string {
	return name + "-lets-encrypt-staging"
}
