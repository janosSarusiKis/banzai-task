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
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CustomIngressLabel      = "feladat.banzaicloud.io/ingress"
	CustomIngressLabelValue = "secure"
	DomainLabel             = "domain"
	EmailLabel              = "email"
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

		var existingIngressPointer, innerError = GetIngressAddressByServiceNameName(r, req.NamespacedName.Name)

		if innerError != nil {
			return ctrl.Result{}, innerError
		}

		if existingIngressPointer != nil {
			r.Delete(ctx, existingIngressPointer)
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var currentIngresses v1beta1.IngressList
	if err := r.List(ctx, &currentIngresses); err != nil {
		return ctrl.Result{}, err
	}

	if customIngressLabelValue, ok := service.ObjectMeta.Labels[CustomIngressLabel]; ok {
		fmt.Println(CustomIngressLabel + " - Service with label was found. Value: " + customIngressLabelValue)
		if customIngressLabelValue == CustomIngressLabelValue {
			var ingress = v1beta1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        service.Name + "-ingress",
					Namespace:   service.Namespace,
					Annotations: map[string]string{"cert-manager.io/cluster-issuer": "test-selfsigned"},
				},
				Spec: v1beta1.IngressSpec{
					TLS: []v1beta1.IngressTLS{
						{
							Hosts:      []string{"example.com"},
							SecretName: service.Name + "-secret",
						},
					},
					Rules: []v1beta1.IngressRule{
						{
							Host: "test.com",
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

			fmt.Println("Try to create ingress")

			var existingIngressPointer, innerError = GetIngressAddressByServiceNameName(r, service.Name)

			if innerError != nil {
				return ctrl.Result{}, innerError
			}

			if existingIngressPointer != nil {
				return ctrl.Result{}, nil
			}

			if err := r.Create(ctx, &ingress); err != nil {
				log.Error(err, "unable to create the Ingress")
				// we'll ignore not-found errors, since they can't be fixed by an immediate
				// requeue (we'll need to wait for a new notification), and we can get them
				// on deleted requests.
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}

			fmt.Println("Ingress created")
		}
	}

	return ctrl.Result{}, nil
}

func (r *CustomIngressManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}

func GetIngressAddressByServiceNameName(r *CustomIngressManagerReconciler, serviceName string) (*v1beta1.Ingress, error) {

	ctx := context.Background()
	var currentIngresses v1beta1.IngressList
	if err := r.List(ctx, &currentIngresses); err != nil {
		return nil, err
	}

	for i := range currentIngresses.Items {
		if currentIngresses.Items[i].Spec.Backend.ServiceName == serviceName {
			fmt.Println("Ingress already there")

			var ingress v1beta1.Ingress = currentIngresses.Items[i]

			return &ingress, nil
		}
	}

	return nil, nil
}
