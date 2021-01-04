package argocd

import (
	"testing"

	"gotest.tools/assert"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func Test_allowedNamespace(t *testing.T) {
	type args struct {
		current        string
		configuredList string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			"a * list",
			args{
				current:        "foo",
				configuredList: "*",
			},
			true,
		},
		{
			"a * list",
			args{
				current:        "foo",
				configuredList: "3456f789",
			},
			false,
		},
		{
			"a long list 1",
			args{
				current:        "foo",
				configuredList: "foo,bar,barfoo",
			},
			true,
		},
		{
			"a long list 2",
			args{
				current:        "foo",
				configuredList: "bar,foo,barfoo",
			},
			true,
		},
		{
			"a long list with value absent",
			args{
				current:        "foo1",
				configuredList: "bar,foo,barfoo",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := allowedNamespace(tt.args.current, tt.args.configuredList); got != tt.want {
				t.Errorf("allowedNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetClusterRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	createClusterRoles(t, r.client)

	serverClusterRole := "argocd-server"
	clusterRole, err := r.getClusterRole(serverClusterRole)
	assertNoError(t, err)
	assert.Equal(t, serverClusterRole, clusterRole.Name)

	controllerClusterRole := "argocd-application-controller"
	clusterRole, err = r.getClusterRole(controllerClusterRole)
	assertNoError(t, err)
	assert.Equal(t, controllerClusterRole, clusterRole.Name)
}
