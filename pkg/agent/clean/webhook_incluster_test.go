//go:build !windows

package clean

import (
	"context"
	"errors"
	"testing"

	"github.com/rancher/rancher/pkg/controllers/management/usercontrollers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCleanupWebhookResources(t *testing.T) {
	binding := &rbacv1.ClusterRoleBinding{Name: usercontrollers.WebhookClusterRoleBindingName}
	mutating := &admissionv1.MutatingWebhookConfiguration{Name: usercontrollers.WebhookConfigurationName}
	validating := &admissionv1.ValidatingWebhookConfiguration{Name: usercontrollers.WebhookConfigurationName}
	errBoom := errors.New("boom")

	tests := []struct {
		name        string
		dryRun      bool
		existing    []runtime.Object
		failDelete  string // resource whose delete returns errBoom
		wantErrs    int
		wantDeleted []string
	}{
		{
			name:        "all three resources are deleted",
			existing:    []runtime.Object{binding, mutating, validating},
			wantDeleted: []string{"clusterrolebindings", "mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
		},
		{
			name:        "missing resources are not errors",
			wantDeleted: []string{"clusterrolebindings", "mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
		},
		{
			name:        "other delete errors are collected and cleanup continues",
			existing:    []runtime.Object{binding, mutating, validating},
			failDelete:  "mutatingwebhookconfigurations",
			wantErrs:    1,
			wantDeleted: []string{"clusterrolebindings", "mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
		},
		{
			name:     "dry run deletes nothing",
			dryRun:   true,
			existing: []runtime.Object{binding, mutating, validating},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The test changes the package variable dryRun, so it must not run in parallel.
			pgSetDryRun(t, tc.dryRun)

			client := fake.NewSimpleClientset(tc.existing...)
			var deleted []string
			client.PrependReactor("delete", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
				resource := action.GetResource().Resource
				deleted = append(deleted, resource)
				if resource == tc.failDelete {
					return true, nil, errBoom
				}
				return false, nil, nil
			})

			errs := cleanupWebhookResources(client)
			assert.Len(t, errs, tc.wantErrs)
			assert.Equal(t, tc.wantDeleted, deleted)

			if tc.wantErrs == 0 && !tc.dryRun {
				_, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), usercontrollers.WebhookClusterRoleBindingName, metav1.GetOptions{})
				require.True(t, apierrors.IsNotFound(err), "the cluster role binding must be gone")
			}
		})
	}
}

// The in-cluster wrappers fail early when the process does not run in a pod.
// The tests check that they stop without a panic and without a Kubernetes call.

func TestRemoveUnusedCattleCredentials_NotInCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	err := removeUnusedCattleCredentials(context.Background())
	require.Error(t, err)
}

func TestUnusedPullSecrets_NotInCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")

	assert.NotPanics(t, func() { UnusedPullSecrets(context.Background()) })
}
