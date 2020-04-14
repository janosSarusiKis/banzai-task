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
	"reflect"
	"regexp"

	isd "github.com/jbenet/go-is-domain"
	"github.com/prometheus/common/log"

	"github.com/go-logr/logr"
	cmacme "github.com/jetstack/cert-manager/pkg/apis/acme/v1alpha3"
	v1alpha3 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha3"
	cmeta1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DomainAnnotation        = "domain"
	EmailAnnotation         = "email"
	CustomIngressLabel      = "feladat.banzaicloud.io/ingress"
	CustomIngressLabelValue = "secure"
	EnvironmentLabel        = "environment"
)

// CustomIngressManagerReconciler reconciles a CustomIngressManager object
type CustomIngressManagerReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webapp.feladat.banzaicloud.io,resources=customingressmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.feladat.banzaicloud.io,resources=customingressmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services;ingresses;clusterissuers,verbs=get;list;create;update;delete,watch
// +kubebuilder:rbac:groups=extensions;cert-manager.io,resources=services;ingresses;clusterissuers,verbs=get;list;create;update;watch

func (r *CustomIngressManagerReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("customingressmanager", req.NamespacedName)

	var service corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &service); err != nil {
		log.Info("service notnd: " + req.Name + " in " + req.Namespace + " namespace")

		existingIngress, err := r.GetIngressByName(CreateIngressName(req.NamespacedName.Name), req.NamespacedName.Namespace)
		if err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		if existingIngress != nil {
			log.Info("deleting existing ingress")
			if err := r.Delete(ctx, existingIngress); err != nil {
				return ctrl.Result{}, err
			}
		}

		existingClusterIssuer, err := r.GetClusterIssuerByName(CreateClusterIssuerName(req.NamespacedName.Name))
		if err != nil && !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		if existingClusterIssuer != nil {
			log.Info("deleting existing cluster issuer")
			if err := r.Delete(ctx, existingClusterIssuer); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if r.IsValidService(&service) {
		log.Info("check if ingress already exists")
		existingIngress, err := r.GetIngressByName(CreateIngressName(service.Name), service.ObjectMeta.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}

		log.Info("check if clusterissuer already exists")
		existingClusterIssuer, err := r.GetClusterIssuerByName(CreateClusterIssuerName(service.Name))
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := r.CreateOrUpdateClusterIssuerForService(service, existingClusterIssuer); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		if err := r.CreateOrUpdateIngressForService(service, existingIngress); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *CustomIngressManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}

func (r *CustomIngressManagerReconciler) GetIngressByName(ingressName, namespace string) (*v1beta1.Ingress, error) {
	ctx := context.Background()
	ingress := v1beta1.Ingress{}
	namespacedName := types.NamespacedName{
		Name:      ingressName,
		Namespace: namespace,
	}

	if err := r.Get(ctx, namespacedName, &ingress); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	r.Log.Info("Ingress already there")

	return &ingress, nil
}

func (r *CustomIngressManagerReconciler) GetClusterIssuerByName(clusterIssuerName string) (*v1alpha3.ClusterIssuer, error) {
	ctx := context.Background()
	clusterIssuer := v1alpha3.ClusterIssuer{}
	namespacedName := types.NamespacedName{
		Name: clusterIssuerName,
	}

	if err := r.Get(ctx, namespacedName, &clusterIssuer); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	r.Log.Info("ClusterIssuer already there")

	return &clusterIssuer, nil
}

func (r *CustomIngressManagerReconciler) IsValidService(service *corev1.Service) bool {
	regExValidaton := regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

	r.Log.Info("validating service")
	if labelValue, ok := service.ObjectMeta.Labels[CustomIngressLabel]; !ok || labelValue != CustomIngressLabelValue {
		r.Log.Info("no custom label")

		return false
	}

	if annotationValue, ok := service.ObjectMeta.Annotations[DomainAnnotation]; !ok || !isd.IsDomain(annotationValue) {
		r.Log.Info("invalid domain name: " + annotationValue)

		return false
	}

	if annotationValue, ok := service.ObjectMeta.Annotations[EmailAnnotation]; !ok || !regExValidaton.MatchString(annotationValue) {
		r.Log.Info("invalid email address: " + annotationValue)

		return false
	}

	r.Log.Info("valid service found")

	return true
}

func (r *CustomIngressManagerReconciler) CreateOrUpdateIngressForService(service corev1.Service, existingIngress *v1beta1.Ingress) error {
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
					Hosts:      []string{service.ObjectMeta.Annotations[DomainAnnotation]},
					SecretName: CreateSecretName(service.Namespace),
				},
			},
			Rules: []v1beta1.IngressRule{
				{
					Host: service.ObjectMeta.Annotations[DomainAnnotation],
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

	if existingIngress != nil {
		if !reflect.DeepEqual(existingIngress, ingress) {
			log.Info("updating Ingress")
			if err := r.Update(ctx, &ingress); err != nil {
				log.Error(err, "unable to update the Ingress")
				// we'll ignore not-found errors, since they can't be fixed by an immediate
				// requeue (we'll need to wait for a new notification), and we can get them
				// on deleted requests.
				return client.IgnoreNotFound(err)
			}
		}

		return nil
	}

	log.Info("try to create Ingress")
	if err := r.Create(ctx, &ingress); err != nil {
		log.Error(err, "unable to create the Ingress")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return client.IgnoreNotFound(err)
	}

	log.Info("ingress created")

	return nil
}

func (r *CustomIngressManagerReconciler) CreateOrUpdateClusterIssuerForService(service corev1.Service, existingClusterIssuer *v1alpha3.ClusterIssuer) error {
	ctx := context.Background()

	var letsencryptUrl string
	if service.ObjectMeta.Annotations[EnvironmentLabel] == "production" {
		letsencryptUrl = "https://acme-v02.api.letsencrypt.org/directory"
	} else {
		letsencryptUrl = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}

	clusterIssuer := v1alpha3.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name: CreateClusterIssuerName(service.Name),
		},
		Spec: v1alpha3.IssuerSpec{
			IssuerConfig: v1alpha3.IssuerConfig{
				ACME: &cmacme.ACMEIssuer{
					Server: letsencryptUrl,
					Email:  service.ObjectMeta.Annotations[EmailAnnotation],
					PrivateKey: cmeta1.SecretKeySelector{
						LocalObjectReference: cmeta1.LocalObjectReference{
							Name: CreateSecretName(service.Namespace),
						},
					},
					Solvers: []cmacme.ACMEChallengeSolver{
						{
							HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
								// Not setting the Class or Name field will cause cert-manager to create
								// new ingress resources that do not specify a class to solve challenges,
								// which means all Ingress controllers should act on the ingresses.
								Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{},
							},
						},
					},
				},
			},
		},
	}

	if existingClusterIssuer != nil {
		if !reflect.DeepEqual(existingClusterIssuer.ObjectMeta.Name, clusterIssuer.ObjectMeta.Name) ||
			!reflect.DeepEqual(existingClusterIssuer.Namespace, clusterIssuer.Namespace) ||
			!reflect.DeepEqual(existingClusterIssuer.Spec.ACME.Email, clusterIssuer.Spec.ACME.Email) {
			log.Info("updating ClusterIssuer")
			if err := r.Update(ctx, &clusterIssuer); err != nil {
				log.Error(err, "unable to update the ClusterIssuer")
				// we'll ignore not-found errors, since they can't be fixed by an immediate
				// requeue (we'll need to wait for a new notification), and we can get them
				// on deleted requests.
				return client.IgnoreNotFound(err)
			}
		}

		return nil
	}

	r.Log.Info("try to create ClusterIssuer")
	if err := r.Create(ctx, &clusterIssuer); err != nil {
		log.Error(err, "unable to create the ClusterIssuer")
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return client.IgnoreNotFound(err)
	}

	return nil
}

func CreateIngressName(name string) string {
	return name + "-ingress"
}

func CreateClusterIssuerName(name string) string {
	return name + "-lets-encrypt-staging"
}

func CreateSecretName(name string) string {
	return name + "-secret"
}
