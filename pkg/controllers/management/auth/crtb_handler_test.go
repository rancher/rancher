package auth

import (
	"fmt"
	"testing"
	"time"

	v31 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacfakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/rancher/rancher/pkg/user"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	// yes, both are needed, used. gh -> wrangler mock, uber -> local mock
	gomockgh "github.com/golang/mock/gomock"
	"go.uber.org/mock/gomock"

	"github.com/rancher/rancher/pkg/controllers/status"
	"github.com/rancher/rancher/pkg/controllers/status/crtb"
)

// using a subset of condition, because we don't need to check LastTransitionTime or Message
type reducedCondition struct {
	reason string
	status v1.ConditionStatus
}

// local implementation of the clusterRoleTemplateBindingController interface for mocking
type clusterRoleTemplateBindingControllerMock struct {
	err error
}

func (m *clusterRoleTemplateBindingControllerMock) UpdateStatus(b *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	return b, m.err
}

var (
	e           = fmt.Errorf("error")
	defaultCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "test",
		GroupName:          "",
		GroupPrincipalName: "",
		ClusterName:        "clusterName",
		RoleTemplateName:   "roleTemplate",
	}
	ensureUserCRTB = v3.ClusterRoleTemplateBinding{
		UserName:          "",
		UserPrincipalName: "tester",
	}
	labeledCRTB = v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				RtbCrbRbLabelsUpdated: "true",
			},
		},
	}
	namedCRTB = v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	noUserCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "",
		GroupName:          "",
		GroupPrincipalName: "",
	}
	statusCRTB = v3.ClusterRoleTemplateBinding{
		Status: v31.ClusterRoleTemplateBindingStatus{
			Conditions: []v1.Condition{
				{
					Status:  v1.ConditionTrue,
					Message: "ok",
					Reason:  "success",
				},
			},
		},
	}
	badStatusCRTB = v3.ClusterRoleTemplateBinding{
		Status: v31.ClusterRoleTemplateBindingStatus{
			Conditions: []v1.Condition{
				{
					Status:  v1.ConditionFalse,
					Message: "bad",
					Reason:  "fail",
				},
			},
		},
	}
	userCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "test",
		UserPrincipalName:  "tester",
		GroupName:          "",
		GroupPrincipalName: "",
		ClusterName:        "clusterName",
		RoleTemplateName:   "roleTemplate",
	}
	userListerCRTB = v3.ClusterRoleTemplateBinding{
		UserName:          "test",
		UserPrincipalName: "",
	}
	defaultCluster = v3.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-cluster",
		},
	}
	defaultClusterRoleBinding = rbacv1.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-crb",
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
	defaultRoleBinding = rbacv1.RoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-rb",
		},
	}
	defaultUser = v3.User{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-user",
		},
	}
)

type crtbTestState struct {
	clusterListerMock *fakes.ClusterListerMock
	crbClientMock     *rbacfakes.ClusterRoleBindingInterfaceMock
	crbListerMock     *rbacfakes.ClusterRoleBindingListerMock
	crtbClientMock    *fakes.ClusterRoleTemplateBindingInterfaceMock
	projectListerMock *fakes.ProjectListerMock
	rbClientMock      *rbacfakes.RoleBindingInterfaceMock
	rbListerMock      *rbacfakes.RoleBindingListerMock
	userListerMock    *fakes.UserListerMock

	managerMock *MockmanagerInterface
	userMGRMock *user.MockManager

	crtbClientMMock *clusterRoleTemplateBindingControllerMock
}

func TestSetCRTBAsInProgress(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		wantError  bool
		crtb       *v3.ClusterRoleTemplateBinding
	}{
		{
			name:      "update failed",
			wantError: true,
			crtb:      statusCRTB.DeepCopy(),
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMMock.err = e
			},
		},
		{
			name: "update ok",
			crtb: statusCRTB.DeepCopy(),
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
			crtbLifecycle.crtbClientM = state.crtbClientMMock

			err := crtbLifecycle.setCRTBAsInProgress(test.crtb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, test.crtb.Status.Conditions, 0)
			}
		})
	}
}

func TestSetCRTBAsCompleted(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		wantError  bool
		crtb       *v3.ClusterRoleTemplateBinding
		condition  reducedCondition
		summary    string
	}{
		{
			name:      "update failed",
			wantError: true,
			crtb:      statusCRTB.DeepCopy(),
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMMock.err = e
			},
		},
		{
			name:      "update success - ok conditions",
			crtb:      statusCRTB.DeepCopy(),
			condition: reducedCondition{reason: "success", status: "True"},
			summary:   status.SummaryCompleted,
		},
		{
			name:      "update success - fail conditions",
			crtb:      badStatusCRTB.DeepCopy(),
			condition: reducedCondition{reason: "fail", status: "False"},
			summary:   status.SummaryError,
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
			crtbLifecycle.crtbClientM = state.crtbClientMMock

			err := crtbLifecycle.setCRTBAsCompleted(test.crtb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, test.crtb.Status.Conditions, 1)
				require.Equal(t, test.condition, rcOf(test.crtb.Status.Conditions[0]))
				require.Equal(t, test.summary, test.crtb.Status.Summary)
			}
		})
	}
}

func TestSetCRTBAsTerminating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		oldCRTB        *v3.ClusterRoleTemplateBinding
		updateReturn error
		wantError    bool
	}{
		{
			name: "update crtb status to Terminating",
			oldCRTB: &v3.ClusterRoleTemplateBinding{
				Status: v31.ClusterRoleTemplateBindingStatus{
					Summary: status.SummaryCompleted,
					Conditions: []v1.Condition{
						{
							Type:   "test",
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update crtb with empty status to Terminating",
			oldCRTB: &v3.ClusterRoleTemplateBinding{
				Status: v31.ClusterRoleTemplateBindingStatus{},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update crtb with nil status to Terminating",
			oldCRTB:        &v3.ClusterRoleTemplateBinding{},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update crtb fails",
			oldCRTB:        &v3.ClusterRoleTemplateBinding{},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			crtbLifecycle := crtbLifecycle{}
			ctrl := gomockgh.NewController(t)

			crtbClientMock := fake.NewMockNonNamespacedControllerInterface[*v3.ClusterRoleTemplateBinding, *v31.ClusterRoleTemplateBindingList](ctrl)

			var updatedCRTB *v3.ClusterRoleTemplateBinding

			crtbClientMock.EXPECT().UpdateStatus(gomock.Any()).AnyTimes().DoAndReturn(
				func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					updatedCRTB = crtb
					return updatedCRTB, test.updateReturn
				},
			)
			crtbLifecycle.crtbClientM = crtbClientMock

			err := crtbLifecycle.setCRTBAsTerminating(test.oldCRTB)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, updatedCRTB.Status.Conditions)
			require.Equal(t, status.SummaryTerminating, updatedCRTB.Status.Summary)
		})
	}
}

func TestReconcileBindings(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		wantError  bool
		crtb       *v3.ClusterRoleTemplateBinding
		condition  reducedCondition
	}{
		{
			name:      "reconcile crtb with no subject",
			crtb:      noUserCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.LocalBindingsExist, status: "True"},
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
			condition: reducedCondition{reason: crtb.FailedToGetCluster, status: "False"},
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
			condition: reducedCondition{reason: crtb.FailedToGetCluster, status: "False"},
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
			condition: reducedCondition{reason: crtb.BadRoleReferences, status: "False"},
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
			condition: reducedCondition{reason: crtb.FailedToEnsureClusterMembership, status: "False"},
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
			condition: reducedCondition{reason: crtb.FailedToGrantManagementPlanePrivileges, status: "False"},
		},
		{
			name: "error listing projects - namespace",
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
			condition: reducedCondition{reason: crtb.FailedToGetNamespace, status: "False"},
		},
		{
			name: "error listing projects - in grantManagementClusterScopedPrivilegesInProjectNamespace",
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
			condition: reducedCondition{reason: crtb.FailedToGrantManagementClusterPrivileges, status: "False"},
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
			crtb:      defaultCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.LocalBindingsExist, status: "True"},
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
			crtb:      defaultCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.LocalBindingsExist, status: "True"},
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
			crtb:      defaultCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.LocalBindingsExist, status: "True"},
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

			require.Len(t, test.crtb.Status.Conditions, 1)
			require.Equal(t, test.condition, rcOf(test.crtb.Status.Conditions[0]))
		})
	}
}

func TestReconcileSubject(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		wantError  bool
		crtb       *v3.ClusterRoleTemplateBinding
		condition  reducedCondition
	}{
		{
			name:      "reconcile crtb with subjects",
			crtb:      userCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.SubjectExists, status: "True"},
		},
		{
			name:      "reconcile crtb with no subjects",
			crtb:      noUserCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: crtb.FailedToGetSubject, status: "False"},
		},
		{
			name:      "error in EnsureUser",
			crtb:      ensureUserCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: crtb.FailedToGetSubject, status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.userMGRMock.EXPECT().
					EnsureUser(gomock.Any(), gomock.Any()).
					Return(nil, e)
			},
		},
		{
			name:      "success EnsureUser",
			crtb:      ensureUserCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.SubjectExists, status: "True"},
			stateSetup: func(cts crtbTestState) {
				cts.userMGRMock.EXPECT().
					EnsureUser(gomock.Any(), gomock.Any()).
					Return(defaultUser.DeepCopy(), nil)
			},
		},
		{
			name:      "error listing user",
			wantError: true,
			crtb:      userListerCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.FailedToGetSubject, status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
					return nil, e
				}
			},
		},
		{
			name:      "success listing user",
			crtb:      userListerCRTB.DeepCopy(),
			condition: reducedCondition{reason: crtb.SubjectExists, status: "True"},
			stateSetup: func(cts crtbTestState) {
				cts.userListerMock.GetFunc = func(namespace, name string) (*v3.User, error) {
					return defaultUser.DeepCopy(), nil
				}
			},
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

			crtbLifecycle.userMGR = state.userMGRMock
			crtbLifecycle.userLister = state.userListerMock

			err := crtbLifecycle.reconcileSubject(test.crtb)

			if test.wantError {
				require.Error(t, err)
				// error, check modified inbound object
				require.Len(t, test.crtb.Status.Conditions, 1)
				require.Equal(t, test.condition, rcOf(test.crtb.Status.Conditions[0]))
			} else {
				require.NoError(t, err)
				// no error, check returned object
				require.Len(t, test.crtb.Status.Conditions, 1)
				require.Equal(t, test.condition, rcOf(test.crtb.Status.Conditions[0]))
			}
		})
	}
}

func TestReconcileLabels(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		wantError  bool
		crtb       *v3.ClusterRoleTemplateBinding
		condition  []reducedCondition
	}{
		{
			name:      "reconciled",
			crtb:      labeledCRTB.DeepCopy(),
			condition: []reducedCondition{{reason: crtb.LocalLabelsSet, status: "True"}},
		},
		{
			name:      "error getting label requirements",
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.LocalFailedToGetLabelRequirements, status: "False"}},
		},
		{
			name:      "error listing cluster role bindings",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.LocalFailedToGetClusterRoleBindings, status: "False"}},
			stateSetup: func(cts crtbTestState) {
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return nil, e
				}
			},
		},
		{
			name:      "error retrieving cluster role binding (for update)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{
				{reason: crtb.LocalFailedToUpdateClusterRoleBindings, status: "False"},
				{reason: crtb.FailedToUpdateRoleBindings, status: "False"},
			},
			stateSetup: func(cts crtbTestState) {
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					crb := defaultClusterRoleBinding.DeepCopy()
					return []*rbacv1.ClusterRoleBinding{crb}, nil
				}
				cts.crbClientMock.GetFunc = func(name string, opts v1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
					return nil, e
				}

				// *** ATTENTION ***
				//
				// The reconcileLabels does not immediately return when it runs into
				// issues with CRBs. It looks for and processes RBs as well, and
				// then reports all collected issues reporting. This means that we
				// have mock the RB ops here also, for success (actually skip), to
				// get the condition we want.

				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return []*rbacv1.RoleBinding{}, nil
				}
			},
		},
		{
			name:      "error updating cluster role binding",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{
				{reason: crtb.LocalFailedToUpdateClusterRoleBindings, status: "False"},
				{reason: crtb.FailedToUpdateRoleBindings, status: "False"},
			},
			stateSetup: func(cts crtbTestState) {
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					crb := defaultClusterRoleBinding.DeepCopy()
					return []*rbacv1.ClusterRoleBinding{crb}, nil
				}
				cts.crbClientMock.GetFunc = func(name string, opts v1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
					return defaultClusterRoleBinding.DeepCopy(), nil
				}
				cts.crbClientMock.UpdateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** As before, skip RB processing
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return []*rbacv1.RoleBinding{}, nil
				}
			},
		},
		{
			name:      "error listing role bindings",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.FailedToGetRoleBindings, status: "False"}},
			stateSetup: func(cts crtbTestState) {
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB processing
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return []*rbacv1.ClusterRoleBinding{}, nil
				}
			},
		},
		{
			name:      "error retrieving role binding (for update)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.FailedToUpdateRoleBindings, status: "False"}},
			stateSetup: func(cts crtbTestState) {
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					rb := defaultRoleBinding.DeepCopy()
					return []*rbacv1.RoleBinding{rb}, nil
				}
				cts.rbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*rbacv1.RoleBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB processing
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return []*rbacv1.ClusterRoleBinding{}, nil
				}
			},
		},
		{
			name:      "error updating role binding",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.FailedToUpdateRoleBindings, status: "False"}},
			stateSetup: func(cts crtbTestState) {
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					rb := defaultRoleBinding.DeepCopy()
					return []*rbacv1.RoleBinding{rb}, nil
				}
				cts.rbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*rbacv1.RoleBinding, error) {
					return defaultRoleBinding.DeepCopy(), nil
				}
				cts.rbClientMock.UpdateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB processing
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return []*rbacv1.ClusterRoleBinding{}, nil
				}
			},
		},
		{
			name:      "error getting crtb (for update)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.LocalFailedToUpdateCRTBLabels, status: "False"}},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB, RB processing
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return []*rbacv1.ClusterRoleBinding{}, nil
				}
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return []*rbacv1.RoleBinding{}, nil
				}
			},
		},
		{
			name:      "error update crtb (labels)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: []reducedCondition{{reason: crtb.LocalFailedToUpdateCRTBLabels, status: "False"}},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return namedCRTB.DeepCopy(), nil
				}
				cts.crtbClientMock.UpdateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB, RB processing
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return []*rbacv1.ClusterRoleBinding{}, nil
				}
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return []*rbacv1.RoleBinding{}, nil
				}
			},
		},
		{
			name:      "success",
			crtb:      namedCRTB.DeepCopy(),
			condition: []reducedCondition{{reason: crtb.LocalLabelsSet, status: "True"}},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return namedCRTB.DeepCopy(), nil
				}
				cts.crtbClientMock.UpdateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					return crtb, nil
				}
				// *** ATTENTION *** Skip CRB, RB processing
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					return []*rbacv1.ClusterRoleBinding{}, nil
				}
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					return []*rbacv1.RoleBinding{}, nil
				}
			},
		},
		{
			name:      "success again with RB and CRB processing",
			crtb:      namedCRTB.DeepCopy(),
			condition: []reducedCondition{{reason: crtb.LocalLabelsSet, status: "True"}},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return namedCRTB.DeepCopy(), nil
				}
				cts.crtbClientMock.UpdateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					return crtb, nil
				}
				cts.crbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.ClusterRoleBinding, error) {
					crb := defaultClusterRoleBinding.DeepCopy()
					return []*rbacv1.ClusterRoleBinding{crb}, nil
				}
				cts.crbClientMock.GetFunc = func(name string, opts v1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
					return defaultClusterRoleBinding.DeepCopy(), nil
				}
				cts.crbClientMock.UpdateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					return crb, nil
				}
				cts.rbListerMock.ListFunc = func(namespace string, selector labels.Selector) ([]*rbacv1.RoleBinding, error) {
					rb := defaultRoleBinding.DeepCopy()
					return []*rbacv1.RoleBinding{rb}, nil
				}
				cts.rbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*rbacv1.RoleBinding, error) {
					return defaultRoleBinding.DeepCopy(), nil
				}
				cts.rbClientMock.UpdateFunc = func(rb *rbacv1.RoleBinding) (*rbacv1.RoleBinding, error) {
					return rb, nil
				}
			},
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

			crtbLifecycle.crbLister = state.crbListerMock
			crtbLifecycle.crbClient = state.crbClientMock
			crtbLifecycle.rbLister = state.rbListerMock
			crtbLifecycle.rbClient = state.rbClientMock
			crtbLifecycle.crtbClient = state.crtbClientMock

			err := crtbLifecycle.reconcileLabels(test.crtb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, test.crtb.Status.Conditions, len(test.condition))
			for idx, c := range test.condition {
				require.Equal(t, c, rcOf(test.crtb.Status.Conditions[idx]))
			}
		})
	}
}

func setupTest(t *testing.T) crtbTestState {
	ctrl := gomock.NewController(t)
	fakeManager := NewMockmanagerInterface(ctrl)
	fakeUserMGR := user.NewMockManager(ctrl)

	clusterListerMock := fakes.ClusterListerMock{}
	projectListerMock := fakes.ProjectListerMock{}
	userListerMock := fakes.UserListerMock{}
	crbClientMock := rbacfakes.ClusterRoleBindingInterfaceMock{}
	crbListerMock := rbacfakes.ClusterRoleBindingListerMock{}
	crtbClientMock := fakes.ClusterRoleTemplateBindingInterfaceMock{}
	rbClientMock := rbacfakes.RoleBindingInterfaceMock{}
	rbListerMock := rbacfakes.RoleBindingListerMock{}
	crtbClientMMock := &clusterRoleTemplateBindingControllerMock{}

	state := crtbTestState{
		clusterListerMock: &clusterListerMock,
		crbClientMock:     &crbClientMock,
		crbListerMock:     &crbListerMock,
		crtbClientMock:    &crtbClientMock,
		projectListerMock: &projectListerMock,
		rbClientMock:      &rbClientMock,
		rbListerMock:      &rbListerMock,
		userListerMock:    &userListerMock,

		userMGRMock: fakeUserMGR,
		managerMock: fakeManager,

		crtbClientMMock: crtbClientMMock,
	}
	return state
}

// rcOf is an internal helper to convert full kube conditions into the reduced
// form used by the tests. This drops the message and lastTransitionTime.
func rcOf(c v1.Condition) reducedCondition {
	return reducedCondition{
		reason: c.Reason,
		status: c.Status,
	}
}
