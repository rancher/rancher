package auth

import (
	"testing"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/rbac/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

// TODO Replace this with generated mocks
type fakeManager struct {
}

func (f *fakeManager) reconcileClusterMembershipBindingForDelete(_, _ string) error {
	return nil
}

func (f *fakeManager) reconcileProjectMembershipBindingForDelete(_, _, _ string) error {
	return nil
}

func (f *fakeManager) checkReferencedRoles(_, _ string, _ int) (bool, error) {
	return true, nil
}

func (f *fakeManager) removeAuthV2Permissions(_ string, _ runtime.Object) error {
	return nil
}

func (f *fakeManager) ensureClusterMembershipBinding(_, _ string, _ *v3.Cluster, _ bool, _ v1.Subject) error {
	return nil
}

func (f *fakeManager) ensureProjectMembershipBinding(_, _, _ string, _ *v3.Project, _ bool, _ v1.Subject) error {
	return nil
}

func (f *fakeManager) grantManagementPlanePrivileges(_ string, _ map[string]string, _ v1.Subject, _ interface{}) error {
	return nil
}

func (f *fakeManager) grantManagementClusterScopedPrivilegesInProjectNamespace(_, _ string, _ map[string]string, _ v1.Subject, _ *v3.ClusterRoleTemplateBinding) error {
	return nil
}

func (f *fakeManager) grantManagementProjectScopedPrivilegesInClusterNamespace(_, _ string, _ map[string]string, _ v1.Subject, _ *v3.ProjectRoleTemplateBinding) error {
	return nil
}

type crtbTestState struct {
	clusterListerMock *fakes.ClusterListerMock
	projectListerMock *fakes.ProjectListerMock
	managerMock       fakeManager
}

func TestReconcileBindings(t *testing.T) {
	tests := []struct {
		name      string
		wantError bool
		crtb      *v3.ClusterRoleTemplateBinding
	}{
		//ADD TESTS
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			crtbLifecycle := crtbLifecycle{}
			state := setupTest(t)

			crtbLifecycle.clusterLister = state.clusterListerMock
			crtbLifecycle.projectLister = state.projectListerMock
			crtbLifecycle.mgr = &state.managerMock

			err := crtbLifecycle.reconcileBindings(test.crtb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func setupTest(t *testing.T) crtbTestState {
	//ctrl := gomock.NewController(t)
	fakeManager := fakeManager{}
	projectListerMock := fakes.ProjectListerMock{}
	clusterListerMock := fakes.ClusterListerMock{}

	state := crtbTestState{
		managerMock:       fakeManager,
		clusterListerMock: &clusterListerMock,
		projectListerMock: &projectListerMock,
	}
	return state
}
