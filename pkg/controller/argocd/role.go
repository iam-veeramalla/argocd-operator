package argocd

import (
	"context"
	"fmt"
	"os"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
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
func (r *ReconcileArgoCD) reconcileRoles(cr *argoprojv1a1.ArgoCD) error {
	if _, err := r.reconcileRole(applicationController, policyRuleForApplicationController(), cr); err != nil {
		return err
	}

	if _, err := r.reconcileRole(dexServer, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	if _, err := r.reconcileRole(server, policyRuleForServer(), cr); err != nil {
		return err
	}

	if _, err := r.reconcileRole(redisHa, policyRuleForRedisHa(), cr); err != nil {
		return err
	}

	rules := []v1.PolicyRule{}

	if cr.Spec.ManagementScope.Cluster != nil && *cr.Spec.ManagementScope.Cluster {
		if os.Getenv("ARGOCD_CLUSTER_CONFIG_ENABLED") == "true" &&
			allowedNamespace(cr.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
			rules = append(rules, policyRoleForClusterConfig()...)
		}
	}

	rules = append(rules, policyRuleForApplicationControllerClusterRole()...)
	if err := r.reconcileClusterRole(applicationController, rules, cr); err != nil {
		return err
	}

	if err := r.reconcileClusterRole(server, policyRuleForServerClusterRole(), cr); err != nil {
		return err
	}

	return nil
}

// reconcileClusterRole
func (r *ReconcileArgoCD) reconcileClusterRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) error {
	rbacClient := r.kc.RbacV1()

	role, err := rbacClient.ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
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

func (r *ReconcileArgoCD) getClusterRole(name string) (*v1.ClusterRole, error) {
	clusterRole := &v1.ClusterRole{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: name}, clusterRole) //rbacClient.ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
	return clusterRole, err
}

func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {

	role := newRoleWithName(name, cr)
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role) //rbacClient.Roles(cr.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	roleExists := true

	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("Failed to reconcile the role for the service account associated with %s : %s", name, err)
		}
		roleExists = false
		role = newRoleWithName(name, cr)
	}

	role.Rules = policyRules

	controllerutil.SetControllerReference(cr, role, r.scheme)
	if roleExists {
		err = r.client.Update(context.TODO(), role)
	} else {
		err = r.client.Create(context.TODO(), role)
	}
	return role, err
}
