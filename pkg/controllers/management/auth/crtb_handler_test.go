package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/controllers/status"
	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
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
	mockTime := time.Unix(0, 0)
	oldTimeNow := timeNow
	timeNow = func() time.Time {
		return mockTime
	}
	t.Cleanup(func() {
		timeNow = oldTimeNow
	})
	mockStatus := &status.Status{
		TimeNow: timeNow,
	}
	tests := []struct {
		name           string
		crtb           *v3.ClusterRoleTemplateBinding
		stateSetup     func(crtbTestState)
		wantError      bool
		wantConditions []v1.Condition
	}{
		{
			name: "reconcile crtb with no subject",
			crtb: noUserCRTB.DeepCopy(),
			wantConditions: []v1.Condition{
				{
					Type:   bindingExists,
					Status: v1.ConditionTrue,
					Reason: bindingExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  failedToGetCluster,
					Message: e.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  clusterNotFound,
					Message: "cannot create binding because cluster clusterName was not found",
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  failedToCheckReferencedRole,
					Message: e.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  failedToEnsureClusterMembershipBinding,
					Message: e.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  failedToGrantManagementPlanePrivileges,
					Message: e.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  failedToListProjects,
					Message: e.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
		},
		{
			name: "error granting management cluster scoped privileges in project namespace",
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
			wantConditions: []v1.Condition{
				{
					Type:    bindingExists,
					Status:  v1.ConditionFalse,
					Reason:  failedToGrantManagementClusterScopedPrivilegesInProjectNamespace,
					Message: e.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:   bindingExists,
					Status: v1.ConditionTrue,
					Reason: bindingExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:   bindingExists,
					Status: v1.ConditionTrue,
					Reason: bindingExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
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
			wantConditions: []v1.Condition{
				{
					Type:   bindingExists,
					Status: v1.ConditionTrue,
					Reason: bindingExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
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
			crtbLifecycle.clusterLister = state.clusterListerMock
			crtbLifecycle.projectLister = state.projectListerMock
			crtbLifecycle.mgr = state.managerMock
			crtbLifecycle.s = mockStatus
			conditions := []v1.Condition{}

			err := crtbLifecycle.reconcileBindings(test.crtb, &conditions)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.wantConditions, conditions)
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	mockTime := time.Unix(0, 0)
	oldTimeNow := timeNow
	timeNow = func() time.Time {
		return mockTime
	}
	t.Cleanup(func() {
		timeNow = oldTimeNow
	})
	ctrl := gomock.NewController(t)

	crtbSubjectExist := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []v1.Condition{
				{
					Type:   subjectExists,
					Status: v1.ConditionTrue,
					Reason: subjectExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbSubjectAndBindingExist := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []v1.Condition{
				{
					Type:   subjectExists,
					Status: v1.ConditionTrue,
					Reason: subjectExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
				{
					Type:   bindingExists,
					Status: v1.ConditionTrue,
					Reason: bindingExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbSubjectError := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LocalConditions: []v1.Condition{
				{
					Type:   subjectExists,
					Status: v1.ConditionFalse,
					Reason: failedToListProjects,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbEmptyStatus := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.Format(time.RFC3339),
		},
	}
	crtbEmptyStatusRemoteComplete := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.Format(time.RFC3339),
			SummaryRemote:  status.SummaryCompleted,
		},
	}
	tests := map[string]struct {
		crtb            *v3.ClusterRoleTemplateBinding
		crtbClient      func(*v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController
		localConditions []v1.Condition
		wantErr         error
	}{
		"status updated": {
			crtb: crtbEmptyStatus.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionTrue,
								Reason: subjectExists,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryCompleted,
					},
				})

				return mock
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
		"status not updated when local conditions are the same": {
			crtb: crtbSubjectExist.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				return fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
		"set summary to complete when remote is complete": {
			crtb: crtbEmptyStatusRemoteComplete.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionTrue,
								Reason: subjectExists,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryCompleted,
						SummaryRemote:  status.SummaryCompleted,
						Summary:        status.SummaryCompleted,
					},
				})

				return mock
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
		"set summary to error when there is an error condition": {
			crtb: crtbSubjectExist.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionFalse,
								Reason: failedToListProjects,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryError,
						Summary:        status.SummaryError,
					},
				})

				return mock
			},
			localConditions: crtbSubjectError.Status.LocalConditions,
		},
		"status updated when a condition is removed": {
			crtb: crtbSubjectAndBindingExist.DeepCopy(),
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().UpdateStatus(&v3.ClusterRoleTemplateBinding{
					Status: v3.ClusterRoleTemplateBindingStatus{
						LocalConditions: []v1.Condition{
							{
								Type:   subjectExists,
								Status: v1.ConditionTrue,
								Reason: subjectExists,
								LastTransitionTime: v1.Time{
									Time: mockTime,
								},
							},
						},
						LastUpdateTime: mockTime.Format(time.RFC3339),
						SummaryLocal:   status.SummaryCompleted,
					},
				})

				return mock
			},
			localConditions: crtbSubjectExist.Status.LocalConditions,
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			crtbCache := fake.NewMockCacheInterface[*v3.ClusterRoleTemplateBinding](ctrl)
			crtbCache.EXPECT().Get(test.crtb.Namespace, test.crtb.Name).Return(test.crtb, nil)
			c := crtbLifecycle{
				crtbClient: test.crtbClient(test.crtb),
				crtbCache:  crtbCache,
			}
			err := c.updateStatus(test.crtb, test.localConditions)
			assert.Equal(t, test.wantErr, err)
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
