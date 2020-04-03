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
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
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
		// TODO: Add test cases.
		{
			name: "valid",
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
		// TODO: Add test cases.
		{
			name: "valid",
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
			name: "invalidEmail",
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
			name: "invalidDomain",
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
			name: "noValidLabe",
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
