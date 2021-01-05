package argocd

import (
	"context"
	"fmt"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	p := policyRuleForApplicationController()

	workloadIdentifier := "xrb"

	assert.NilError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &v1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))

	// undesirable changes
	roleBinding.RoleRef.Name = "not-xrb"
	roleBinding.Subjects[0].Name = "not-xrb"
	assert.NilError(t, r.client.Update(context.TODO(), roleBinding))

	// try reconciling it again to ensure undesirable changes are overwritten
	assert.NilError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding = &v1.RoleBinding{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))

}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedClusterRole := &v1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}}
	expectedServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier, Namespace: a.Namespace}}

	assert.NilError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, expectedServiceAccount, a))

	clusterRoleBinding := &v1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("cluster-%s-%s", a.Name, workloadIdentifier)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// undesirable changes
	clusterRoleBinding.RoleRef.Name = "not-x"
	clusterRoleBinding.Subjects[0].Name = "not-x"
	assert.NilError(t, r.client.Update(context.TODO(), clusterRoleBinding))

	// try reconciling it again to ensure undesirable changes are overwritten
	assert.NilError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, expectedServiceAccount, a))

	clusterRoleBinding = &v1.ClusterRoleBinding{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))
}
