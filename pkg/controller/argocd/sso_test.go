// Copyright 2021 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocd

import (
	"context"
	"testing"

	argov1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"gotest.tools/assert"
	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func makeFakeReconciler(t *testing.T, acd *argov1alpha1.ArgoCD, objs ...runtime.Object) *ReconcileArgoCD {
	t.Helper()
	s := scheme.Scheme
	s.AddKnownTypes(argov1alpha1.SchemeGroupVersion, acd)
	// Register template scheme
	s.AddKnownTypes(templatev1.SchemeGroupVersion, objs...)
	s.AddKnownTypes(oappsv1.SchemeGroupVersion, objs...)
	templatev1.Install(s)
	oappsv1.Install(s)
	routev1.Install(s)

	cl := fake.NewFakeClientWithScheme(s, objs...)
	return &ReconcileArgoCD{
		client: cl,
		scheme: s,
	}
}

func TestReconcile_testKeycloakTemplateInstance(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	templateAPIFound = true
	r := makeFakeReconciler(t, a)

	assert.NilError(t, r.reconcileSSO(a))

	templateInstance := &templatev1.TemplateInstance{}
	assert.NilError(t, r.client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "rhsso",
			Namespace: a.Namespace,
		},
		templateInstance))
}

func TestReconcile_noTemplateInstance(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDForKeycloak()
	r := makeFakeReconciler(t, a)

	assert.NilError(t, r.reconcileSSO(a))
}

func TestReconcile_testKeycloakK8sInstance(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	// Cluster does not have a template instance
	templateAPIFound = false
	r := makeReconciler(t, a)

	assert.NilError(t, r.reconcileSSO(a))
}

func TestReconcile_testKeycloakInstanceResources(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	// Cluster does not have a template instance
	templateAPIFound = false
	r := makeReconciler(t, a)

	assert.NilError(t, r.reconcileSSO(a))

	// Keycloak Deployment
	deployment := &k8sappsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: a.Namespace}, deployment)
	assert.NilError(t, err)

	assert.Equal(t, deployment.Name, defaultKeycloakIdentifier)
	assert.Equal(t, deployment.Namespace, a.Namespace)

	testLabels := map[string]string{
		"app": defaultKeycloakIdentifier,
	}
	assert.DeepEqual(t, deployment.Labels, testLabels)

	testSelector := &v1.LabelSelector{
		MatchLabels: map[string]string{
			"app": defaultKeycloakIdentifier,
		},
	}
	assert.DeepEqual(t, deployment.Spec.Selector, testSelector)

	assert.DeepEqual(t, deployment.Spec.Template.ObjectMeta.Labels, testLabels)
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Name,
		defaultKeycloakIdentifier)
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Image,
		defaultKeycloakImage)

	testEnv := []corev1.EnvVar{
		{Name: "KEYCLOAK_USER", Value: defaultKeycloakAdminUser},
		{Name: "KEYCLOAK_PASSWORD", Value: defaultKeycloakAdminPassword},
		{Name: "PROXY_ADDRESS_FORWARDING", Value: "true"},
	}
	assert.DeepEqual(t, deployment.Spec.Template.Spec.Containers[0].Env,
		testEnv)

	// Keycloak Service
	svc := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: a.Namespace}, svc)
	assert.NilError(t, err)

	assert.Equal(t, svc.Name, defaultKeycloakIdentifier)
	assert.Equal(t, svc.Namespace, a.Namespace)
	assert.DeepEqual(t, svc.Labels, testLabels)

	assert.DeepEqual(t, svc.Spec.Selector, testLabels)
	assert.DeepEqual(t, svc.Spec.Type, corev1.ServiceType("LoadBalancer"))

	// Keycloak Ingress
	ing := &extv1beta1.Ingress{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: a.Namespace}, ing)
	assert.NilError(t, err)

	assert.Equal(t, ing.Name, defaultKeycloakIdentifier)
	assert.Equal(t, ing.Namespace, a.Namespace)

	testTLS := []extv1beta1.IngressTLS{
		{
			Hosts: []string{keycloakIngressHost},
		},
	}
	assert.DeepEqual(t, ing.Spec.TLS, testTLS)

	testRules := []extv1beta1.IngressRule{
		{
			Host: keycloakIngressHost,
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Backend: extv1beta1.IngressBackend{
								ServiceName: defaultKeycloakIdentifier,
								ServicePort: intstr.FromInt(int(httpPort)),
							},
						},
					},
				},
			},
		},
	}

	assert.DeepEqual(t, ing.Spec.Rules, testRules)
}
