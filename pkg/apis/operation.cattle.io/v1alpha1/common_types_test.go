package v1alpha1

import (
	"testing"

	"github.com/rancher/wrangler/v3/pkg/condition"
	corev1 "k8s.io/api/core/v1"
)

func TestClusterRefKey(t *testing.T) {
	tests := []struct {
		name string
		ref  *corev1.ObjectReference
		want string
	}{
		{
			name: "namespaced reference",
			ref: &corev1.ObjectReference{
				APIVersion: "provisioning.cattle.io/v1",
				Kind:       "Cluster",
				Namespace:  "fleet-default",
				Name:       "test",
			},
			want: "apiVersion=provisioning.cattle.io/v1, kind=Cluster, namespace=fleet-default, name=test",
		},
		{
			// Imported clusters reference the cluster-scoped mgmt v3 Cluster, which has no
			// namespace to report.
			name: "cluster-scoped reference omits the namespace",
			ref: &corev1.ObjectReference{
				APIVersion: "management.cattle.io/v3",
				Kind:       "Cluster",
				Name:       "c-m-abcde",
			},
			want: "apiVersion=management.cattle.io/v3, kind=Cluster, name=c-m-abcde",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClusterRefKey(tt.ref); got != tt.want {
				t.Fatalf("ClusterRefKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutcomeConditionFor(t *testing.T) {
	tests := []struct {
		name        string
		phase       OperationPhase
		wantCond    condition.Cond
		wantSummary string
	}{
		{
			name:        "succeeded",
			phase:       OperationPhaseSucceeded,
			wantCond:    SucceededCondition,
			wantSummary: "Operation completed successfully",
		},
		{
			name:        "failed",
			phase:       OperationPhaseFailed,
			wantCond:    FailedCondition,
			wantSummary: "Operation failed",
		},
		{
			name:        "canceled",
			phase:       OperationPhaseCanceled,
			wantCond:    CanceledCondition,
			wantSummary: "Operation canceled",
		},
		{
			// Callers check the phase is terminal first; an unrecognised one is treated as a
			// failure, matching how the operation controllers handle it.
			name:        "non-terminal falls back to failed",
			phase:       OperationPhaseInProgress,
			wantCond:    FailedCondition,
			wantSummary: "Operation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond, summary := OutcomeConditionFor(tt.phase)
			if cond != tt.wantCond {
				t.Fatalf("condition = %q, want %q", cond, tt.wantCond)
			}
			if summary != tt.wantSummary {
				t.Fatalf("summary = %q, want %q", summary, tt.wantSummary)
			}
		})
	}
}
