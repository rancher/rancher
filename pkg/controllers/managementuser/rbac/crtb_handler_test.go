package rbac

import (
	"fmt"
	"testing"

	v31 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	rbacfakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		UserName:         "crtb-name",
		RoleTemplateName: "rt-name",
	}
	labeledCRTB = v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Labels: map[string]string{
				rtbCrbRbLabelsUpdated: "true",
			},
		},
	}
	namedCRTB = v3.ClusterRoleTemplateBinding{
		ObjectMeta: v1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
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
	defaultClusterRoleBinding = rbacv1.ClusterRoleBinding{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-crb",
		},
	}
)

type crtbTestState struct {
	managerMock     *MockmanagerInterface
	crbClientMock   *rbacfakes.ClusterRoleBindingInterfaceMock
	crtbClientMock  *fakes.ClusterRoleTemplateBindingInterfaceMock
	rtListerMock    *fakes.RoleTemplateListerMock
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
			state := setupCRTBTest(t)
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
			summary:   SummaryCompleted,
		},
		{
			name:      "update success - fail conditions",
			crtb:      badStatusCRTB.DeepCopy(),
			condition: reducedCondition{reason: "fail", status: "False"},
			summary:   SummaryError,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			crtbLifecycle := crtbLifecycle{}
			state := setupCRTBTest(t)
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

func TestSyncCRTB(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		crtb       *v3.ClusterRoleTemplateBinding
		wantError  bool
		condition  reducedCondition
	}{
		{
			name:      "crtb with no role template",
			crtb:      noRoleTemplateCRTB.DeepCopy(),
			condition: reducedCondition{reason: "NoBindingsRequired", status: "True"},
		},
		{
			name:      "crtb with no subject",
			crtb:      noSubjectCRTB.DeepCopy(),
			condition: reducedCondition{reason: "NoBindingsRequired", status: "True"},
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
			condition: reducedCondition{reason: "FailedToGetRoleTemplate", status: "False"},
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
			condition: reducedCondition{reason: "FailedToGetRoles", status: "False"},
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
			condition: reducedCondition{reason: "FailedToEnsureRoles", status: "False"},
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
			condition: reducedCondition{reason: "FailedToEnsureClusterRoleBindings", status: "False"},
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
			condition: reducedCondition{reason: "FailedToEnsureSAImpersonator", status: "False"},
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
			crtb:      defaultCRTB.DeepCopy(),
			condition: reducedCondition{reason: "BindingsExist", status: "True"},
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

			err := crtbLifecycle.syncCRTB(test.crtb)

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

func TestReconcileCRTBUserClusterLabels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		stateSetup func(crtbTestState)
		crtb       *v3.ClusterRoleTemplateBinding
		wantError  bool
		condition  reducedCondition
	}{
		{
			name:      "reconciled",
			crtb:      labeledCRTB.DeepCopy(),
			condition: reducedCondition{reason: "LabelsSet", status: "True"},
		},
		// error getting label requirements -- does not look to be inducable
		{
			name:      "error listing cluster role bindings",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: "FailedToGetClusterRoleBindings", status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					return nil, e
				}
			},
		},
		{
			name:      "error retrieving cluster role binding (for update)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: "FailedToUpdateClusterRoleBindings", status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					crb := defaultClusterRoleBinding.DeepCopy()
					return &rbacv1.ClusterRoleBindingList{
						Items: []rbacv1.ClusterRoleBinding{*crb},
					}, nil
				}
				cts.crbClientMock.GetFunc = func(name string, opts v1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
					return nil, e
				}
			},
		},
		{
			name:      "error updating cluster role binding",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: "FailedToUpdateClusterRoleBindings", status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					crb := defaultClusterRoleBinding.DeepCopy()
					return &rbacv1.ClusterRoleBindingList{
						Items: []rbacv1.ClusterRoleBinding{*crb},
					}, nil
				}
				cts.crbClientMock.GetFunc = func(name string, opts v1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
					return defaultClusterRoleBinding.DeepCopy(), nil
				}
				cts.crbClientMock.UpdateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					return nil, e
				}
			},
		},
		{
			name:      "error getting crtb (for update)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: "FailedToUpdateCRTBLabels", status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB processing
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					return &rbacv1.ClusterRoleBindingList{}, nil
				}
			},
		},
		{
			name:      "error update crtb (labels)",
			crtb:      namedCRTB.DeepCopy(),
			wantError: true,
			condition: reducedCondition{reason: "FailedToUpdateCRTBLabels", status: "False"},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return namedCRTB.DeepCopy(), nil
				}
				cts.crtbClientMock.UpdateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					return nil, e
				}
				// *** ATTENTION *** Skip CRB processing
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					return &rbacv1.ClusterRoleBindingList{}, nil
				}
			},
		},
		{
			name:      "success",
			crtb:      namedCRTB.DeepCopy(),
			condition: reducedCondition{reason: "LabelsSet", status: "True"},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return namedCRTB.DeepCopy(), nil
				}
				cts.crtbClientMock.UpdateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					return crtb, nil
				}
				// *** ATTENTION *** Skip CRB processing
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					return &rbacv1.ClusterRoleBindingList{}, nil
				}
			},
		},
		{
			name:      "success again with CRB processing",
			crtb:      namedCRTB.DeepCopy(),
			condition: reducedCondition{reason: "LabelsSet", status: "True"},
			stateSetup: func(cts crtbTestState) {
				cts.crtbClientMock.GetNamespacedFunc = func(namespace, name string, opts v1.GetOptions) (*v3.ClusterRoleTemplateBinding, error) {
					return namedCRTB.DeepCopy(), nil
				}
				cts.crtbClientMock.UpdateFunc = func(crtb *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
					return crtb, nil
				}
				cts.crbClientMock.ListFunc = func(opts v1.ListOptions) (*rbacv1.ClusterRoleBindingList, error) {
					crb := defaultClusterRoleBinding.DeepCopy()
					return &rbacv1.ClusterRoleBindingList{
						Items: []rbacv1.ClusterRoleBinding{*crb},
					}, nil
				}
				cts.crbClientMock.GetFunc = func(name string, opts v1.GetOptions) (*rbacv1.ClusterRoleBinding, error) {
					return defaultClusterRoleBinding.DeepCopy(), nil
				}
				cts.crbClientMock.UpdateFunc = func(crb *rbacv1.ClusterRoleBinding) (*rbacv1.ClusterRoleBinding, error) {
					return crb, nil
				}
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

			crtbLifecycle.crbClient = state.crbClientMock
			crtbLifecycle.crtbClient = state.crtbClientMock

			err := crtbLifecycle.reconcileCRTBUserClusterLabels(test.crtb)

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

func setupCRTBTest(t *testing.T) crtbTestState {
	ctrl := gomock.NewController(t)
	fakeManager := NewMockmanagerInterface(ctrl)

	crbClientMock := rbacfakes.ClusterRoleBindingInterfaceMock{}
	crtbClientMock := fakes.ClusterRoleTemplateBindingInterfaceMock{}
	rtListerMock := fakes.RoleTemplateListerMock{}

	crtbClientMMock := &clusterRoleTemplateBindingControllerMock{}

	state := crtbTestState{
		managerMock: fakeManager,

		crbClientMock:  &crbClientMock,
		crtbClientMock: &crtbClientMock,
		rtListerMock:   &rtListerMock,

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
