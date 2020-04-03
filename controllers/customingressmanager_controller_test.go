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
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	v1alpha3 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientFaker "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateClusterIssuerName(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Valid",
			args: args{
				name: "svc",
			},
			want: "svc-lets-encrypt-staging",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreateClusterIssuerName(tt.args.name); got != tt.want {
				t.Errorf("CreateClusterIssuerName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCustomIngressManagerReconciler_IsValidService(t *testing.T) {
	type fields struct {
		Client client.Client
		Log    logr.Logger
		Scheme *runtime.Scheme
	}
	type args struct {
		service *corev1.Service
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Valid",
			fields: fields{
				Client: clientFaker.NewFakeClient(),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				service: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "testsvc",
						Namespace:   "default",
						Annotations: map[string]string{"domain": "test.com", "email": "tes@test.com"},
						Labels:      map[string]string{"feladat.banzaicloud.io/ingress": "secure"},
					},
				},
			},
			want: true,
		},
		{
			name: "InvalidEmail",
			fields: fields{
				Client: clientFaker.NewFakeClient(),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				service: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "testsvc",
						Namespace:   "default",
						Annotations: map[string]string{"domain": "test.com", "email": "test"},
						Labels:      map[string]string{"feladat.banzaicloud.io/ingress": "secure"},
					},
				},
			},
			want: false,
		},
		{
			name: "InvalidDomain",
			fields: fields{
				Client: clientFaker.NewFakeClient(),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				service: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "testsvc",
						Namespace:   "default",
						Annotations: map[string]string{"domain": "test", "email": "test@test.com"},
						Labels:      map[string]string{"feladat.banzaicloud.io/ingress": "secure"},
					},
				},
			},
			want: false,
		},
		{
			name: "NoValidLabe",
			fields: fields{
				Client: clientFaker.NewFakeClient(),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				service: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "testsvc",
						Namespace:   "default",
						Annotations: map[string]string{"domain": "test.com", "email": "tes"},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CustomIngressManagerReconciler{
				Client: tt.fields.Client,
				Log:    tt.fields.Log,
				Scheme: tt.fields.Scheme,
			}
			if got := r.IsValidService(tt.args.service); got != tt.want {
				t.Errorf("CustomIngressManagerReconciler.IsValidService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCustomIngressManagerReconciler_GetIngressByName(t *testing.T) {
	testIngress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testsvc-ingress",
			Namespace: "test",
		},
	}

	testIngresses := &v1beta1.IngressList{
		Items: []v1beta1.Ingress{
			*testIngress,
		},
	}

	testScheme := runtime.NewScheme()
	_ = v1alpha3.AddToScheme(testScheme)
	_ = v1beta1.AddToScheme(testScheme)

	type fields struct {
		Client client.Client
		Log    logr.Logger
		Scheme *runtime.Scheme
	}
	type args struct {
		ingressName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *v1beta1.Ingress
		wantErr bool
	}{
		{
			name: "Valid",
			fields: fields{
				Client: clientFaker.NewFakeClientWithScheme(testScheme, testIngresses),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				ingressName: "testsvc-ingress",
			},
			want:    testIngress,
			wantErr: false,
		},
		{
			name: "NoIngress",
			fields: fields{
				Client: clientFaker.NewFakeClientWithScheme(testScheme),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				ingressName: "testsvc-ingress",
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CustomIngressManagerReconciler{
				Client: tt.fields.Client,
				Log:    tt.fields.Log,
				Scheme: tt.fields.Scheme,
			}

			got, err := r.GetIngressByName(tt.args.ingressName)
			if (err != nil) != tt.wantErr {
				t.Errorf("CustomIngressManagerReconciler.GetIngressByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CustomIngressManagerReconciler.GetIngressByName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCustomIngressManagerReconciler_CreateIngressForService(t *testing.T) {
	type fields struct {
		Client client.Client
		Log    logr.Logger
		Scheme *runtime.Scheme
	}
	type args struct {
		service corev1.Service
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Valid",
			fields: fields{
				Client: clientFaker.NewFakeClient(),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: runtime.NewScheme(),
			},
			args: args{
				service: corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "testsvc",
						Namespace:   "default",
						Annotations: map[string]string{"domain": "test.com", "email": "tes@test.com"},
						Labels:      map[string]string{"feladat.banzaicloud.io/ingress": "secure"},
					},
				},
			},

			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CustomIngressManagerReconciler{
				Client: tt.fields.Client,
				Log:    tt.fields.Log,
				Scheme: tt.fields.Scheme,
			}
			if err := r.CreateIngressForService(tt.args.service); (err != nil) != tt.wantErr {
				t.Errorf("CustomIngressManagerReconciler.CreateIngressForService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCustomIngressManagerReconciler_CreateClusterIssuerForService(t *testing.T) {
	testScheme := runtime.NewScheme()
	_ = v1alpha3.AddToScheme(testScheme)

	type fields struct {
		Client client.Client
		Log    logr.Logger
		Scheme *runtime.Scheme
	}
	type args struct {
		service corev1.Service
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Valid",
			fields: fields{
				Client: clientFaker.NewFakeClientWithScheme(testScheme),
				Log:    ctrl.Log.WithName("customingressmanager"),
				Scheme: testScheme,
			},
			args: args{
				service: corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "testsvc",
						Namespace:   "default",
						Annotations: map[string]string{"domain": "test.com", "email": "tes@test.com"},
						Labels:      map[string]string{"feladat.banzaicloud.io/ingress": "secure"},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CustomIngressManagerReconciler{
				Client: tt.fields.Client,
				Log:    tt.fields.Log,
				Scheme: tt.fields.Scheme,
			}
			if err := r.CreateClusterIssuerForService(tt.args.service); (err != nil) != tt.wantErr {
				t.Errorf("CustomIngressManagerReconciler.CreateClusterIssuerForService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
