package argocd

import (
	"context"
	"fmt"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

func (r *ReconcileArgoCD) getClusterRole(name string) (*v1.ClusterRole, error) {
	clusterRoleList := &v1.ClusterRoleList{}
	err := r.client.List(context.TODO(), clusterRoleList)
	if err != nil {
		return nil, err
	}

	for _, clusterRole := range clusterRoleList.Items {
		if strings.Contains(clusterRole.Name, name) {
			return &clusterRole, nil
		}
	}

	return nil, fmt.Errorf("Unable to find ClusterRole: %s", name)
}

// newRoleWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	sa := newRole(name, cr)
	sa.Name = name

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

// newRole returns a new ServiceAccount instance.
func newRole(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newClusterRoleWithName creates a new ClusterRole with the given name for the given ArgCD.
func newClusterRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
	sa := newClusterRole(name, cr)
	sa.Name = generateResourceName(name, cr)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

// newClusterRole returns a new ClusterRole instance.
func newClusterRole(name string, cr *argoprojv1a1.ArgoCD) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateResourceName(name, cr),
			Labels: map[string]string{},
		},
	}
}

func allowedNamespace(current string, configuredList string) bool {
	isAllowedNamespace := false
	if configuredList != "" {
		if configuredList == "*" {
			isAllowedNamespace = true
		} else {
			namespaceList := strings.Split(configuredList, ",")
			for _, n := range namespaceList {
				if n == current {
					isAllowedNamespace = true
				}
			}
		}
	}
	return isAllowedNamespace
}

// reconcileRoles will ensure that all ArgoCD Service Accounts are configured.
func (r *ReconcileArgoCD) reconcileRoles(cr *argoprojv1a1.ArgoCD) (role *v1.Role, error error) {
	if role, err := r.reconcileRole(applicationController, policyRuleForApplicationController(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(dexServer, policyRuleForDexServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(server, policyRuleForServer(), cr); err != nil {
		return role, err
	}

	if role, err := r.reconcileRole(redisHa, policyRuleForRedisHa(), cr); err != nil {
		return role, err
	}

	rules := []v1.PolicyRule{}

	if cr.Spec.ManagementScope.Cluster != nil && *cr.Spec.ManagementScope.Cluster {
		if cr.ObjectMeta.Labels["ARGOCD_CLUSTER_CONFIG_ENABLED"] == "true" &&
			allowedNamespace(cr.Namespace, cr.ObjectMeta.Labels["ARGOCD_CLUSTER_CONFIG_NAMESPACES"]) {
			rules = append(rules, policyRoleForClusterConfig()...)
		}
	}

	rules = append(rules, policyRuleForApplicationControllerClusterRole()...)
	if err := r.reconcileClusterRole(applicationController, rules, cr); err != nil {
		return nil, err
	}

	if err := r.reconcileClusterRole(server, policyRuleForServerClusterRole(), cr); err != nil {
		return nil, err
	}

	return nil, nil
}

// reconcileClusterRole
func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	rbacClient := r.kc.RbacV1()

	role, err := rbacClient.ClusterRoles().Get(context.TODO(), generateResourceName(name, cr), metav1.GetOptions{})
	roleExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleExists = false
			role = newClusterRoleWithName(name, cr)
		} else {
			return err
		}
	}

	role.Rules = policyRules

	if roleExists {
		_, err = rbacClient.ClusterRoles().Update(context.TODO(), role, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err = rbacClient.ClusterRoles().Create(context.TODO(), role, metav1.CreateOptions{})
	}
	return err
}

// reconcileRole
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (role *v1.Role, error error) {
	rbacClient := r.kc.RbacV1()

	role, err := rbacClient.Roles(cr.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
	roleExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleExists = false
			role = newRoleWithName(name, cr)
		} else {
			return role, err
		}
	}

	role.Rules = policyRules

	controllerutil.SetControllerReference(cr, role, r.scheme)
	if roleExists {
		role, err = rbacClient.Roles(cr.Namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
		if err != nil {
			return role, err
		}
	} else {
		role, err = rbacClient.Roles(cr.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	}
	return role, err
}
