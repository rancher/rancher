package status

import (
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterRolesExists        = "ClusterRolesExists"
	clusterRoleBindingsExists = "ClusterRoleBindingsExists"
	failedToCreateRoles       = "FailedToCreateRoles"
)

func TestAddCondition(t *testing.T) {
	mockTime := time.Unix(0, 0)
	mockErr := errors.New("mock error")

	tests := map[string]struct {
		conditions     []v1.Condition
		condition      v1.Condition
		reason         string
		err            error
		wantConditions []v1.Condition
	}{
		"add new condition": {
			conditions: []v1.Condition{},
			condition:  v1.Condition{Type: clusterRolesExists},
			reason:     clusterRolesExists,
			err:        nil,
			wantConditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionTrue,
					Reason: clusterRolesExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
		},
		"add new condition when there are already other existing conditions": {
			conditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionTrue,
					Reason: clusterRolesExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			condition: v1.Condition{Type: clusterRoleBindingsExists},
			reason:    clusterRoleBindingsExists,
			err:       nil,
			wantConditions: []v1.Condition{
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
			},
		},
		"add new condition with error": {
			conditions: nil,
			condition:  v1.Condition{Type: clusterRolesExists},
			reason:     failedToCreateRoles,
			err:        mockErr,
			wantConditions: []v1.Condition{
				{
					Type:    clusterRolesExists,
					Status:  v1.ConditionFalse,
					Message: mockErr.Error(),
					Reason:  failedToCreateRoles,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
		},
		"modify existing condition": {
			conditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionTrue,
					Reason: clusterRolesExists,
					LastTransitionTime: v1.Time{
						Time: time.Now(),
					},
				},
			},
			condition: v1.Condition{Type: clusterRolesExists},
			reason:    clusterRolesExists,
			err:       mockErr,
			wantConditions: []v1.Condition{
				{
					Type:    clusterRolesExists,
					Status:  v1.ConditionFalse,
					Reason:  clusterRolesExists,
					Message: mockErr.Error(),
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
		},
		"modify existing error condition": {
			conditions: []v1.Condition{
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
					Message: mockErr.Error(),
					Reason:  failedToCreateRoles,
					LastTransitionTime: v1.Time{
						Time: time.Now(),
					},
				},
			},
			condition: v1.Condition{Type: clusterRoleBindingsExists},
			reason:    clusterRoleBindingsExists,
			err:       nil,
			wantConditions: []v1.Condition{
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
			},
		},
		"add existing condition": {
			conditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionTrue,
					Reason: clusterRolesExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
			condition: v1.Condition{Type: clusterRolesExists},
			reason:    clusterRolesExists,
			err:       nil,
			wantConditions: []v1.Condition{
				{
					Type:   clusterRolesExists,
					Status: v1.ConditionTrue,
					Reason: clusterRolesExists,
					LastTransitionTime: v1.Time{
						Time: mockTime,
					},
				},
			},
		},
	}

	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := Status{TimeNow: func() time.Time {
				return mockTime
			}}
			conditions := test.conditions
			s.AddCondition(&conditions, test.condition, test.reason, test.err)
			assert.Equal(t, test.wantConditions, conditions)
		})
	}
}

func TestCompareConditions(t *testing.T) {
	tests := map[string]struct {
		s1, s2 []metav1.Condition
		want   bool
	}{
		"equals": {
			s1: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			s2: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			want: true,
		},
		"equals with multiple conditions in different order": {
			s1: []metav1.Condition{
				{
					Type:               clusterRoleBindingsExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			s2: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
				{
					Type:               clusterRoleBindingsExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			want: true,
		},
		"different type": {
			s1: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			s2: []metav1.Condition{
				{
					Type:               clusterRoleBindingsExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			want: false,
		},
		"different status": {
			s1: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			s2: []metav1.Condition{
				{
					Type:               clusterRoleBindingsExists,
					Status:             SummaryError,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			want: false,
		},
		"different len": {
			s1: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             clusterRolesExists,
					Message:            "",
				},
				{
					Type:               clusterRolesExists,
					Status:             SummaryCompleted,
					LastTransitionTime: metav1.Time{Time: time.Now()},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			s2: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             SummaryError,
					LastTransitionTime: metav1.Time{Time: time.Unix(0, 0)},
					Reason:             clusterRolesExists,
					Message:            "",
				},
			},
			want: false,
		},
		"empty slices": {
			s1:   []metav1.Condition{},
			s2:   []metav1.Condition{},
			want: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, CompareConditions(test.s1, test.s2))
		})
	}
}

func TestKeepLastTransitionTimeIfConditionIsPresent(t *testing.T) {
	timeZero := metav1.NewTime(time.Unix(0, 0))
	timeOne := metav1.NewTime(time.Unix(1, 1))
	tests := map[string]struct {
		conditions            []metav1.Condition
		conditionsFromCluster []metav1.Condition
		wantConditions        []metav1.Condition
	}{
		"conditions present in both are updated": {
			conditions: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             clusterRolesExists,
					LastTransitionTime: timeZero,
					Reason:             "reason",
					Message:            "message",
				},
			},
			conditionsFromCluster: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             clusterRolesExists,
					LastTransitionTime: timeOne,
					Reason:             "reason",
					Message:            "message",
				},
			},
			wantConditions: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             clusterRolesExists,
					LastTransitionTime: timeOne,
					Reason:             "reason",
					Message:            "message",
				},
			},
		},
		"conditions not present in both are not updated": {
			conditions: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             clusterRolesExists,
					LastTransitionTime: timeZero,
					Reason:             "reason",
					Message:            "message",
				},
			},
			conditionsFromCluster: []metav1.Condition{
				{
					Type:               clusterRoleBindingsExists,
					Status:             clusterRoleBindingsExists,
					LastTransitionTime: timeOne,
					Reason:             "reason",
					Message:            "message",
				},
			},
			wantConditions: []metav1.Condition{
				{
					Type:               clusterRolesExists,
					Status:             clusterRolesExists,
					LastTransitionTime: timeZero,
					Reason:             "reason",
					Message:            "message",
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			KeepLastTransitionTimeIfConditionHasNotChanged(test.conditions, test.conditionsFromCluster)

			assert.Equal(t, test.wantConditions, test.conditions)
		})
	}

}
