package argocd

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/api/rbac/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	p := policyRuleForApplicationController()

	workloadIdentifier := "xrb"
	expectedServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier, Namespace: a.Namespace}}

	assertNoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &v1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))

	// undesirable changes
	roleBinding.RoleRef.Name = "not-xrb"
	roleBinding.Subjects[0].Name = "not-xrb"
	assertNoError(t, r.client.Update(context.TODO(), roleBinding))

	// try reconciling it again to ensure undesirable changes are overwritten
	assertNoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding = &v1.RoleBinding{}
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))

	assert.Equal(t, expectedServiceAccount.Name, roleBinding.RoleRef.Name)
	assert.Equal(t, expectedServiceAccount.Name, roleBinding.Subjects[0].Name)
}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier, Namespace: a.Namespace}}

	clusterRoleBinding := &v1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// undesirable changes
	clusterRoleBinding.RoleRef.Name = "not-x"
	clusterRoleBinding.Subjects[0].Name = "not-x"
	assertNoError(t, r.client.Update(context.TODO(), clusterRoleBinding))

	// try reconciling it again to ensure undesirable changes are overwritten
	assertNoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, a))

	clusterRoleBinding = &v1.ClusterRoleBinding{}
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	assert.Equal(t, expectedServiceAccount.Name, clusterRoleBinding.RoleRef.Name)
	assert.Equal(t, expectedServiceAccount.Name, clusterRoleBinding.Subjects[0].Name)
}
