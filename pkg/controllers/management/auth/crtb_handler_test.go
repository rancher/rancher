package auth

import (
	"fmt"
	"testing"
	"time"

	apisv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	corefakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	errDefault  = fmt.Errorf("error")
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
	backingNamespaceProject = v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-project",
		},
		Status: apisv3.ProjectStatus{
			BackingNamespace: "c-ABC-p-XYZ",
		},
	}
	deletingProject = v3.Project{
		ObjectMeta: v1.ObjectMeta{
			Name:              "deleting-project",
			DeletionTimestamp: &v1.Time{Time: time.Now()},
		},
	}
	defaultBinding = rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-binding",
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
					return nil, errDefault
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
					Return(true, errDefault)
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
					Return(errDefault)

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
					Return(errDefault)
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
					return nil, errDefault
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
					Return(errDefault)

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
			name: "successfully reconcile clustermember with backingNamespace",
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
					grantManagementClusterScopedPrivilegesInProjectNamespace("roleTemplate", "c-ABC-p-XYZ", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
				cts.clusterListerMock.GetFunc = func(namespace, name string) (*v3.Cluster, error) {
					c := defaultCluster.DeepCopy()
					return c, nil
				}
				cts.projectListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*v3.Project, error) {
					p := backingNamespaceProject.DeepCopy()
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
					Return(errDefault).AnyTimes()
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

func Test_removeMGMTClusterScopedPrivilegesInProjectNamespace(t *testing.T) {
	tests := []struct {
		name                  string
		projectListFunc       func(string, labels.Selector) ([]*apisv3.Project, error)
		roleBindingListFunc   func(string, labels.Selector) ([]*rbacv1.RoleBinding, error)
		roleBindingDeleteFunc func(string, string, *v1.DeleteOptions) error
		binding               *v3.ClusterRoleTemplateBinding
		wantErr               bool
	}{
		{
			name: "error listing projects",
			projectListFunc: func(s1 string, s2 labels.Selector) ([]*apisv3.Project, error) {
				return nil, errDefault
			},
			binding: defaultCRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error listing rolebindings",
			projectListFunc: func(s1 string, s2 labels.Selector) ([]*apisv3.Project, error) {
				return []*apisv3.Project{
					defaultProject.DeepCopy(),
				}, nil
			},
			roleBindingListFunc: func(s1 string, s2 labels.Selector) ([]*rbacv1.RoleBinding, error) {
				return nil, errDefault
			},
			binding: defaultCRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "error deleting rolebindings",
			projectListFunc: func(s1 string, s2 labels.Selector) ([]*apisv3.Project, error) {
				return []*apisv3.Project{
					defaultProject.DeepCopy(),
				}, nil
			},
			roleBindingListFunc: func(s1 string, s2 labels.Selector) ([]*rbacv1.RoleBinding, error) {
				return []*rbacv1.RoleBinding{
					defaultBinding.DeepCopy(),
				}, nil
			},
			roleBindingDeleteFunc: func(s1, s2 string, do *v1.DeleteOptions) error {
				return errDefault
			},
			binding: defaultCRTB.DeepCopy(),
			wantErr: true,
		},
		{
			name: "successfully delete rolebindings no backing namespace",
			projectListFunc: func(s1 string, s2 labels.Selector) ([]*apisv3.Project, error) {
				return []*apisv3.Project{
					defaultProject.DeepCopy(),
				}, nil
			},
			roleBindingListFunc: func(s1 string, s2 labels.Selector) ([]*rbacv1.RoleBinding, error) {
				assert.Equal(t, defaultProject.Name, s1)
				return []*rbacv1.RoleBinding{
					defaultBinding.DeepCopy(),
				}, nil
			},
			roleBindingDeleteFunc: func(s1, s2 string, do *v1.DeleteOptions) error {
				assert.Equal(t, defaultProject.Name, s1)
				return nil
			},
			binding: defaultCRTB.DeepCopy(),
		},
		{
			name: "successfully delete rolebindings with backing namespace",
			projectListFunc: func(s1 string, s2 labels.Selector) ([]*apisv3.Project, error) {
				return []*apisv3.Project{
					backingNamespaceProject.DeepCopy(),
				}, nil
			},
			roleBindingListFunc: func(s1 string, s2 labels.Selector) ([]*rbacv1.RoleBinding, error) {
				assert.Equal(t, backingNamespaceProject.Status.BackingNamespace, s1)
				return []*rbacv1.RoleBinding{
					defaultBinding.DeepCopy(),
				}, nil
			},
			roleBindingDeleteFunc: func(s1, s2 string, do *v1.DeleteOptions) error {
				assert.Equal(t, backingNamespaceProject.Status.BackingNamespace, s1)
				return nil
			},
			binding: defaultCRTB.DeepCopy(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fakes.ProjectListerMock{}
			p.ListFunc = tt.projectListFunc
			rbl := corefakes.RoleBindingListerMock{}
			rbl.ListFunc = tt.roleBindingListFunc
			rbi := corefakes.RoleBindingInterfaceMock{}
			rbi.DeleteNamespacedFunc = tt.roleBindingDeleteFunc

			c := &crtbLifecycle{
				projectLister: &p,
				rbLister:      &rbl,
				rbClient:      &rbi,
			}
			if err := c.removeMGMTClusterScopedPrivilegesInProjectNamespace(tt.binding); (err != nil) != tt.wantErr {
				t.Errorf("crtbLifecycle.removeMGMTClusterScopedPrivilegesInProjectNamespace() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
