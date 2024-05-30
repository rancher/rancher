package auth

import (
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	e           = fmt.Errorf("error")
	defaultCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "test",
		GroupName:          "",
		GroupPrincipalName: "",
		ClusterName:        "clusterName",
		RoleTemplateName:   "roleTemplate",
	}
	noUserCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "",
		GroupName:          "",
		GroupPrincipalName: "",
	}
	defaultCluster = v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-cluster",
		},
	}
	defaultProject = v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-project",
		},
	}
	deletingProject = v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:              "deleting-project",
			DeletionTimestamp: &v1.Time{Time: time.Now()},
		},
	}
)

type crtbTestState struct {
	clusterListerMock *fakes.ClusterListerMock
	projectListerMock *fakes.ProjectListerMock
	managerMock       *MockmanagerInterface
}

func TestReconcileBindings(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		wantError  bool
		crtb       *v3.ClusterRoleTemplateBinding
	}{
		{
			name: "reconcile crtb with no subject",
			crtb: noUserCRTB.DeepCopy(),
		},
		{
			name: "error getting cluster",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					return nil, e
				}
			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "cluster not found",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					return nil, nil
				}
			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "error in checkReferencedRoles",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(true, e)
			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "error in ensureClusterMembershipBinding",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(true, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clusterowner", gomock.Any(), gomock.Any(), true, gomock.Any()).
					Return(e)

			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "error in grantManagementPlanePrivileges",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(true, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clusterowner", gomock.Any(), gomock.Any(), true, gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementPlanePrivileges("roleTemplate", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(e)
			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "error listing projects",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(true, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clusterowner", gomock.Any(), gomock.Any(), true, gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementPlanePrivileges("roleTemplate", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.projectListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Project, error) {
					return nil, e
				}
			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "error listing projects",
			stateSetup: func(cts crtbTestState) {
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(true, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clusterowner", gomock.Any(), gomock.Any(), true, gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementPlanePrivileges("roleTemplate", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.projectListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Project, error) {
					p := defaultProject.DeepCopy()
					return []*v3.Project{p}, nil
				}
				cts.managerMock.EXPECT().
					grantManagementClusterScopedPrivilegesInProjectNamespace("roleTemplate", "test-project", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(e)

			},
			wantError: true,
			crtb:      defaultCRTB.DeepCopy(),
		},
		{
			name: "successfully reconcile clusterowner",
			stateSetup: func(cts crtbTestState) {
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(true, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clusterowner", gomock.Any(), gomock.Any(), true, gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementPlanePrivileges("roleTemplate", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementClusterScopedPrivilegesInProjectNamespace("roleTemplate", "test-project", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.projectListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Project, error) {
					p := defaultProject.DeepCopy()
					return []*v3.Project{p}, nil
				}
			},
			crtb: defaultCRTB.DeepCopy(),
		},
		{
			name: "successfully reconcile clustermember",
			stateSetup: func(cts crtbTestState) {
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(false, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clustermember", gomock.Any(), gomock.Any(), false, gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementPlanePrivileges("roleTemplate", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementClusterScopedPrivilegesInProjectNamespace("roleTemplate", "test-project", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.projectListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Project, error) {
					p := defaultProject.DeepCopy()
					return []*v3.Project{p}, nil
				}
			},
			crtb: defaultCRTB.DeepCopy(),
		},
		{
			name: "skip projects that are deleting",
			stateSetup: func(cts crtbTestState) {
				cts.managerMock.EXPECT().
					checkReferencedRoles("roleTemplate", "cluster", gomock.Any()).
					Return(false, nil)
				cts.managerMock.EXPECT().
					ensureClusterMembershipBinding("clustername-clustermember", gomock.Any(), gomock.Any(), false, gomock.Any()).
					Return(nil)
				cts.managerMock.EXPECT().
					grantManagementPlanePrivileges("roleTemplate", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				// This should not be called
				cts.managerMock.EXPECT().
					grantManagementClusterScopedPrivilegesInProjectNamespace("roleTemplate", "deleting-project", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(e).AnyTimes()
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.projectListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Project, error) {
					p := deletingProject.DeepCopy()
					return []*v3.Project{p}, nil
				}
			},
			crtb: defaultCRTB.DeepCopy(),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			crtbLifecycle := crtbLifecycle{}
			state := setupTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			crtbLifecycle.clusterLister = state.clusterListerMock
			crtbLifecycle.projectLister = state.projectListerMock
			crtbLifecycle.mgr = state.managerMock

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
	ctrl := gomock.NewController(t)
	fakeManager := NewMockmanagerInterface(ctrl)
	projectListerMock := fakes.ProjectListerMock{}
	clusterListerMock := fakes.ClusterListerMock{}

	state := crtbTestState{
		managerMock:       fakeManager,
		clusterListerMock: &clusterListerMock,
		projectListerMock: &projectListerMock,
	}
	return state
}
