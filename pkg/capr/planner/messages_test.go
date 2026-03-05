package planner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

func TestRemoveReconciledCondition(t *testing.T) {
	tests := []struct {
		name                 string
		machine              *capi.Machine
		expectedV1Beta2Conds []metav1.Condition
		expectedV1Beta1Conds capi.Conditions
	}{
		{
			name:    "nil machine returns nil",
			machine: nil,
		},
		{
			name: "no conditions at all",
			machine: &capi.Machine{
				Status: capi.MachineStatus{},
			},
		},
		{
			name: "no Reconciled condition in either location",
			machine: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
						{Type: "PlanApplied", Status: metav1.ConditionTrue, Reason: "PlanApplied"},
					},
					Deprecated: &capi.MachineDeprecatedStatus{
						V1Beta1: &capi.MachineV1Beta1DeprecatedStatus{
							Conditions: capi.Conditions{
								{Type: "Ready", Status: corev1.ConditionTrue},
								{Type: "PlanApplied", Status: corev1.ConditionTrue},
							},
						},
					},
				},
			},
			expectedV1Beta2Conds: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
				{Type: "PlanApplied", Status: metav1.ConditionTrue, Reason: "PlanApplied"},
			},
			expectedV1Beta1Conds: capi.Conditions{
				{Type: "Ready", Status: corev1.ConditionTrue},
				{Type: "PlanApplied", Status: corev1.ConditionTrue},
			},
		},
		{
			name: "Reconciled in v1beta2 only is removed",
			machine: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
						{Type: "Reconciled", Status: metav1.ConditionUnknown, Reason: "Waiting", Message: "waiting for something"},
						{Type: "PlanApplied", Status: metav1.ConditionTrue, Reason: "PlanApplied"},
					},
				},
			},
			expectedV1Beta2Conds: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
				{Type: "PlanApplied", Status: metav1.ConditionTrue, Reason: "PlanApplied"},
			},
		},
		{
			name: "Reconciled in v1beta1 only is removed",
			machine: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
					},
					Deprecated: &capi.MachineDeprecatedStatus{
						V1Beta1: &capi.MachineV1Beta1DeprecatedStatus{
							Conditions: capi.Conditions{
								{Type: "Ready", Status: corev1.ConditionTrue},
								{Type: "Reconciled", Status: corev1.ConditionUnknown, Reason: "Waiting", Message: "waiting for something"},
							},
						},
					},
				},
			},
			expectedV1Beta2Conds: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
			},
			expectedV1Beta1Conds: capi.Conditions{
				{Type: "Ready", Status: corev1.ConditionTrue},
			},
		},
		{
			name: "Reconciled in both v1beta2 and v1beta1 is removed from both",
			machine: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
						{Type: "Reconciled", Status: metav1.ConditionUnknown, Reason: "Waiting", Message: "waiting for something"},
						{Type: "PlanApplied", Status: metav1.ConditionTrue, Reason: "PlanApplied"},
					},
					Deprecated: &capi.MachineDeprecatedStatus{
						V1Beta1: &capi.MachineV1Beta1DeprecatedStatus{
							Conditions: capi.Conditions{
								{Type: "Ready", Status: corev1.ConditionTrue},
								{Type: "Reconciled", Status: corev1.ConditionUnknown, Reason: "Waiting", Message: "waiting for something"},
								{Type: "PlanApplied", Status: corev1.ConditionTrue},
							},
						},
					},
				},
			},
			expectedV1Beta2Conds: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
				{Type: "PlanApplied", Status: metav1.ConditionTrue, Reason: "PlanApplied"},
			},
			expectedV1Beta1Conds: capi.Conditions{
				{Type: "Ready", Status: corev1.ConditionTrue},
				{Type: "PlanApplied", Status: corev1.ConditionTrue},
			},
		},
		{
			name: "Reconciled is the only condition in both locations",
			machine: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{Type: "Reconciled", Status: metav1.ConditionTrue, Reason: "Reconciled"},
					},
					Deprecated: &capi.MachineDeprecatedStatus{
						V1Beta1: &capi.MachineV1Beta1DeprecatedStatus{
							Conditions: capi.Conditions{
								{Type: "Reconciled", Status: corev1.ConditionTrue, Reason: "Reconciled"},
							},
						},
					},
				},
			},
			expectedV1Beta2Conds: []metav1.Condition{},
			expectedV1Beta1Conds: capi.Conditions{},
		},
		{
			name: "Deprecated is nil but Reconciled exists in v1beta2",
			machine: &capi.Machine{
				Status: capi.MachineStatus{
					Conditions: []metav1.Condition{
						{Type: "Reconciled", Status: metav1.ConditionTrue, Reason: "Reconciled"},
						{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
					},
				},
			},
			expectedV1Beta2Conds: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "Ready"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeReconciledCondition(tt.machine)

			if tt.machine == nil {
				assert.Nil(t, result)
				return
			}

			assert.Equal(t, tt.expectedV1Beta2Conds, result.Status.Conditions)
			assert.Equal(t, tt.expectedV1Beta1Conds, result.GetV1Beta1Conditions())
		})
	}
}
