package rbac

import (
	"fmt"
	"testing"

	mgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacfakes "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1/fakes"
	"github.com/stretchr/testify/require"

	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const generation int64 = 1

// local implementation of the globalRoleBindingController interface for mocking
type globalRoleBindingControllerMock struct {
	err error
}

func (m *globalRoleBindingControllerMock) UpdateStatus(b *v3.GlobalRoleBinding) (*v3.GlobalRoleBinding, error) {
	return b, m.err
}

type grbTestState struct {
	crbClientMock *rbacfakes.ClusterRoleBindingInterfaceMock
	crbListerMock *rbacfakes.ClusterRoleBindingListerMock
}

func TestEnsureClusterAdminBinding(t *testing.T) {
	tests := []struct {
		name       string
		stateSetup func(grbTestState)
		grb        *v3.GlobalRoleBinding
		wantError  bool
		condition  reducedCondition
	}{
		{
			name:      "fail to get CRB",
			wantError: true,
			stateSetup: func(gts grbTestState) {
				gts.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("error")
				}
			},
			grb:       &v3.GlobalRoleBinding{},
			condition: reducedCondition{reason: "FailedToGetCRB", status: "False"},
		},
		{
			name:      "CRB exists",
			wantError: false,
			stateSetup: func(gts grbTestState) {
				gts.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return &v1.ClusterRoleBinding{}, nil
				}
			},
			grb:       &v3.GlobalRoleBinding{},
			condition: reducedCondition{reason: "CRBExists", status: "True"},
		},
		{
			name:      "CRB missing, fails to create",
			wantError: true,
			stateSetup: func(gts grbTestState) {
				gts.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group: "g", Resource: "r",
					}, "bogus")
				}
				gts.crbClientMock.CreateFunc = func(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
					return nil, fmt.Errorf("error")
				}
			},
			grb:       &v3.GlobalRoleBinding{},
			condition: reducedCondition{reason: "FailedToCreateCRB", status: "False"},
		},
		{
			name:      "CRB missing, create finds it existing (race), ok",
			wantError: false,
			stateSetup: func(gts grbTestState) {
				gts.crbListerMock.GetFunc = func(namespace, name string) (*v1.ClusterRoleBinding, error) {
					return nil, apierrors.NewNotFound(schema.GroupResource{
						Group: "g", Resource: "r",
					}, "bogus")
				}
				gts.crbClientMock.CreateFunc = func(obj *v1.ClusterRoleBinding) (*v1.ClusterRoleBinding, error) {
					return nil, apierrors.NewAlreadyExists(schema.GroupResource{
						Group: "g", Resource: "r",
					}, "bogus")
				}
			},
			grb:       &v3.GlobalRoleBinding{},
			condition: reducedCondition{reason: "CRBExists", status: "True"},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			grbHandler := grbHandler{}
			state := setupGRBTest(t)
			if test.stateSetup != nil {
				test.stateSetup(state)
			}

			grbHandler.crbLister = state.crbListerMock
			grbHandler.clusterRoleBindings = state.crbClientMock

			err := grbHandler.ensureClusterAdminBinding(test.grb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, test.grb.Status.Conditions, 1)
			require.Equal(t, test.condition, rcOf(test.grb.Status.Conditions[0]))
		})
	}
}

func TestSetGRBAsInProgress(t *testing.T) {
	tests := []struct {
		name         string
		grb          *v3.GlobalRoleBinding
		updateReturn error
		wantError    bool
	}{
		{
			name: "update grb status to InProgress",
			grb: &v3.GlobalRoleBinding{
				Status: mgmtv3.GlobalRoleBindingStatus{
					Summary: SummaryCompleted,
					Conditions: []metav1.Condition{
						{
							Type:   "test",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update grb with empty status to InProgress",
			grb: &v3.GlobalRoleBinding{
				Status: mgmtv3.GlobalRoleBindingStatus{},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update grb with nil status to InProgress",
			grb:          &v3.GlobalRoleBinding{},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update grb fails",
			grb:          &v3.GlobalRoleBinding{},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			grbLifecycle := grbHandler{}
			grbClientMock := &globalRoleBindingControllerMock{
				err: test.updateReturn,
			}
			grbLifecycle.grbClient = grbClientMock

			err := grbLifecycle.setGRBAsInProgress(test.grb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, test.grb.Status.Conditions)
			require.Equal(t, SummaryInProgress, test.grb.Status.Summary)
		})
	}

}

func TestSetGRBAsCompleted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		grb          *v3.GlobalRoleBinding
		summary      string
		updateReturn error
		wantError    bool
	}{
		{
			name: "grb with a met condition is Completed",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleBindingStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "grb with multiple met conditions is Completed",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleBindingStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "grb with no conditions is Completed",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleBindingStatus{
					Conditions: []metav1.Condition{},
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "grb with nil status is Completed",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
			},
			summary:      SummaryCompleted,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "grb with one unmet and one met condition is Error",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleBindingStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionTrue,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			summary:      SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "grb with multiple unmet conditions is Error",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleBindingStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionFalse,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionFalse,
						},
					},
				},
			},
			summary:      SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "grb with unknown conditions is Error",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
				Status: mgmtv3.GlobalRoleBindingStatus{
					Conditions: []metav1.Condition{
						{
							Type:   "test1",
							Status: metav1.ConditionUnknown,
						},
						{
							Type:   "test2",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			summary:      SummaryError,
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update grb fails",
			grb: &v3.GlobalRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Generation: generation,
				},
			},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			grbLifecycle := grbHandler{}
			grbClientMock := &globalRoleBindingControllerMock{
				err: test.updateReturn,
			}
			grbLifecycle.grbClient = grbClientMock

			err := grbLifecycle.setGRBAsCompleted(test.grb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if test.summary != "" {
				require.Equal(t, test.summary, test.grb.Status.Summary)
			}
			require.Equal(t, generation, test.grb.Status.ObservedGeneration)
		})
	}
}

func TestSetGRBAsTerminating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		grb          *v3.GlobalRoleBinding
		updateReturn error
		wantError    bool
	}{
		{
			name: "update grb status to Terminating",
			grb: &v3.GlobalRoleBinding{
				Status: mgmtv3.GlobalRoleBindingStatus{
					Summary: SummaryCompleted,
					Conditions: []metav1.Condition{
						{
							Type:   "test",
							Status: metav1.ConditionTrue,
						},
					},
				},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name: "update grb with empty status to Terminating",
			grb: &v3.GlobalRoleBinding{
				Status: mgmtv3.GlobalRoleBindingStatus{},
			},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update grb with nil status to Terminating",
			grb:          &v3.GlobalRoleBinding{},
			updateReturn: nil,
			wantError:    false,
		},
		{
			name:         "update grb fails",
			grb:          &v3.GlobalRoleBinding{},
			updateReturn: fmt.Errorf("error"),
			wantError:    true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			grbLifecycle := grbHandler{}
			grbClientMock := &globalRoleBindingControllerMock{
				err: test.updateReturn,
			}
			grbLifecycle.grbClient = grbClientMock

			err := grbLifecycle.setGRBAsTerminating(test.grb)

			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, test.grb.Status.Conditions)
			require.Equal(t, SummaryTerminating, test.grb.Status.Summary)
		})
	}
}

func setupGRBTest(t *testing.T) grbTestState {
	crbClientMock := rbacfakes.ClusterRoleBindingInterfaceMock{}
	crbListerMock := rbacfakes.ClusterRoleBindingListerMock{}

	state := grbTestState{
		crbClientMock: &crbClientMock,
		crbListerMock: &crbListerMock,
	}
	return state
}
