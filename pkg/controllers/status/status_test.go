package status

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
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
