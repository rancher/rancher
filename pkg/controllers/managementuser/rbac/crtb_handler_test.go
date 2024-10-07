package rbac

import (
	"fmt"
	"testing"
	"time"

	"github.com/rancher/rancher/pkg/controllers/status"
	controllersv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	e           = fmt.Errorf("error")
	defaultCRTB = v3.ClusterRoleTemplateBinding{
		UserName:         "crtb-name",
		RoleTemplateName: "rt-name",
	}
	noRoleTemplateCRTB = v3.ClusterRoleTemplateBinding{
		UserName:         "crtb-name",
		RoleTemplateName: "",
	}
	noSubjectCRTB = v3.ClusterRoleTemplateBinding{
		UserName:           "",
		GroupName:          "",
		GroupPrincipalName: "",
		RoleTemplateName:   "rt-name",
	}
)

type crtbTestState struct {
	managerMock  *MockmanagerInterface
	rtListerMock *fakes.RoleTemplateListerMock
}

func TestSyncCRTB(t *testing.T) {
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
	t.Parallel()
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		crtb       *v3.ClusterRoleTemplateBinding
		wantError  bool
		wantStatus v3.ClusterRoleTemplateBindingStatus
	}{
		{
			name: "crtb with no role template",
			crtb: noRoleTemplateCRTB.DeepCopy(),
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:   clusterRolesExists,
						Status: v1.ConditionTrue,
						Reason: roleTemplateDoesNotExist,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "crtb with no subject",
			crtb: noSubjectCRTB.DeepCopy(),
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:   clusterRolesExists,
						Status: v1.ConditionTrue,
						Reason: userOrGroupDoesNotExist,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "error getting roletemplate",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, e
				}
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:    clusterRolesExists,
						Status:  v1.ConditionFalse,
						Message: "couldn't get role template rt-name: " + e.Error(),
						Reason:  failedToGetRoleTemplate,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "error gathering roles",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:    clusterRolesExists,
						Status:  v1.ConditionFalse,
						Message: e.Error(),
						Reason:  failedToGatherRoles,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "error ensuring roles",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:    clusterRolesExists,
						Status:  v1.ConditionFalse,
						Message: "couldn't ensure roles: " + e.Error(),
						Reason:  failedToCreateRoles,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "error ensuring cluster bindings",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureClusterBindings(gomock.Any(), gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:   clusterRolesExists,
						Status: v1.ConditionTrue,
						Reason: clusterRolesExists,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
					{
						Type:    clusterRoleBindingsExists,
						Status:  v1.ConditionFalse,
						Message: "couldn't ensure cluster bindings : " + e.Error(),
						Reason:  failedToCreateBindings,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "error ensuring service account impersonator",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureClusterBindings(gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureServiceAccountImpersonator(gomock.Any()).Return(e)
			},
			crtb:      defaultCRTB.DeepCopy(),
			wantError: true,
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:   clusterRolesExists,
						Status: v1.ConditionTrue,
						Reason: clusterRolesExists,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
					{
						Type:   clusterRoleBindingsExists,
						Status: v1.ConditionTrue,
						Reason: clusterRoleBindingsExists,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
					{
						Type:    serviceAccountImpersonatorExists,
						Status:  v1.ConditionFalse,
						Message: "couldn't ensure service account impersonator: " + e.Error(),
						Reason:  failedToCreateServiceAccountImpersonator,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
		{
			name: "success",
			stateSetup: func(cts crtbTestState) {
				cts.rtListerMock.GetFunc = func(namespace, name string) (*v3.RoleTemplate, error) {
					return nil, nil
				}
				cts.managerMock.EXPECT().gatherRoles(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureRoles(gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureClusterBindings(gomock.Any(), gomock.Any()).Return(nil)
				cts.managerMock.EXPECT().ensureServiceAccountImpersonator(gomock.Any()).Return(nil)
			},
			crtb: defaultCRTB.DeepCopy(),
			wantStatus: v3.ClusterRoleTemplateBindingStatus{
				RemoteConditions: []v1.Condition{
					{
						Type:   clusterRolesExists,
						Status: v1.ConditionTrue,
						Reason: clusterRolesExists,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
					{
						Type:   clusterRoleBindingsExists,
						Status: v1.ConditionTrue,
						Reason: clusterRoleBindingsExists,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
					{
						Type:   serviceAccountImpersonatorExists,
						Status: v1.ConditionTrue,
						Reason: serviceAccountImpersonatorExists,
						LastTransitionTime: v1.Time{
							Time: mockTime,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			crtbLifecycle := crtbLifecycle{}
			state := setupCRTBTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}
			crtbLifecycle.rtLister = state.rtListerMock
			crtbLifecycle.m = state.managerMock
			crtbLifecycle.s = mockStatus

			err := crtbLifecycle.syncCRTB(test.crtb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.wantStatus, test.crtb.Status)
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

	crtbClusterRolesExists := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			RemoteConditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionTrue,
					Reason: clusterRolesExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.String(),
		},
	}
	crtbSubjectError := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			RemoteConditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionFalse,
					Reason: failedToCreateRoles,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			LastUpdateTime: mockTime.String(),
		},
	}
	crtbEmptyStatus := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.String(),
		},
	}
	crtbEmptyStatusLocalComplete := &v3.ClusterRoleTemplateBinding{
		Status: v3.ClusterRoleTemplateBindingStatus{
			LastUpdateTime: mockTime.String(),
			SummaryLocal:   status.SummaryCompleted,
		},
	}
	tests := map[string]struct {
		crtb       *v3.ClusterRoleTemplateBinding
		crtbClient func(*v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController
		wantErr    error
	}{
		"status updated": {
			crtb: crtbClusterRolesExists,
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				crtbSubjectExist := crtbClusterRolesExists.DeepCopy()
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().Get(crtb.Namespace, crtb.Name, v1.GetOptions{}).Return(crtbEmptyStatus, nil)
				crtbSubjectExist.Status.SummaryRemote = status.SummaryCompleted
				mock.EXPECT().UpdateStatus(crtbSubjectExist)

				return mock
			},
		},
		"status not updated when remote conditions are the same": {
			crtb: crtbClusterRolesExists,
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().Get(crtb.Namespace, crtb.Name, v1.GetOptions{}).Return(crtbClusterRolesExists.DeepCopy(), nil)

				return mock
			},
		},
		"set summary to complete when local is complete": {
			crtb: crtbClusterRolesExists,
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				crtbSubjectExist := crtbClusterRolesExists.DeepCopy()
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().Get(crtb.Namespace, crtb.Name, v1.GetOptions{}).Return(crtbEmptyStatusLocalComplete, nil)
				crtbSubjectExist.Status.SummaryLocal = status.SummaryCompleted
				crtbSubjectExist.Status.SummaryRemote = status.SummaryCompleted
				crtbSubjectExist.Status.Summary = status.SummaryCompleted
				mock.EXPECT().UpdateStatus(crtbSubjectExist)

				return mock
			},
		},
		"set summary to error when there is an error condition": {
			crtb: crtbSubjectError,
			crtbClient: func(crtb *v3.ClusterRoleTemplateBinding) controllersv3.ClusterRoleTemplateBindingController {
				crtbSubjectExist := crtbClusterRolesExists.DeepCopy()
				mock := fake.NewMockControllerInterface[*v3.ClusterRoleTemplateBinding, *v3.ClusterRoleTemplateBindingList](ctrl)
				mock.EXPECT().Get(crtb.Namespace, crtb.Name, v1.GetOptions{}).Return(crtbSubjectExist, nil)
				crtbSubjectExist.Status.SummaryRemote = status.SummaryError
				crtbSubjectExist.Status.Summary = status.SummaryError
				mock.EXPECT().UpdateStatus(crtbSubjectExist)

				return mock
			},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := crtbLifecycle{
				crtbClient: test.crtbClient(test.crtb),
			}
			err := c.updateStatus(test.crtb)
			assert.Equal(t, test.wantErr, err)
		})
	}
}

func setupCRTBTest(t *testing.T) crtbTestState {
	ctrl := gomock.NewController(t)
	fakeManager := NewMockmanagerInterface(ctrl)
	rtListerMock := fakes.RoleTemplateListerMock{}
	state := crtbTestState{
		managerMock:  fakeManager,
		rtListerMock: &rtListerMock,
	}
	return state
}
