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
			name:        "aborted",
			phase:       OperationPhaseAborted,
			wantCond:    AbortedCondition,
			wantSummary: "Operation aborted",
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

// TestMarkOutcome covers the transitions the operation controllers make when an operation ends. The
// phase and the condition that reports it are asserted together, so a status can never claim one
// outcome by its phase and another by its conditions.
func TestMarkOutcome(t *testing.T) {
	cases := []struct {
		name       string
		mark       func(s *OperationStatus)
		wantPhase  OperationPhase
		wantCond   string
		wantReason string
		wantMsg    string
	}{
		{
			name:       "succeeded",
			mark:       func(s *OperationStatus) { s.MarkSucceeded() },
			wantPhase:  OperationPhaseSucceeded,
			wantCond:   "Succeeded",
			wantReason: FinishedReason,
			wantMsg:    "Operation completed successfully",
		},
		{
			name:       "failed",
			mark:       func(s *OperationStatus) { s.MarkFailed(PlanFailedReason, "the operative detail") },
			wantPhase:  OperationPhaseFailed,
			wantCond:   "Failed",
			wantReason: PlanFailedReason,
			wantMsg:    "the operative detail",
		},
		{
			name:       "aborted",
			mark:       func(s *OperationStatus) { s.MarkAborted(PreflightCheckFailedReason, "the operative detail") },
			wantPhase:  OperationPhaseAborted,
			wantCond:   "Aborted",
			wantReason: PreflightCheckFailedReason,
			wantMsg:    "the operative detail",
		},
		{
			name:       "canceled",
			mark:       func(s *OperationStatus) { s.MarkCanceled(CancelRequestedReason, "cancellation requested") },
			wantPhase:  OperationPhaseCanceled,
			wantCond:   "Canceled",
			wantReason: CancelRequestedReason,
			wantMsg:    "cancellation requested",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := &OperationStatus{}
			tc.mark(status)

			if status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q", status.Phase, tc.wantPhase)
			}
			if status.LastUpdated.IsZero() {
				t.Fatal("the transition must be recorded on LastUpdated")
			}

			outcome, _ := OutcomeConditionFor(status.Phase)
			if string(outcome) != tc.wantCond {
				t.Fatalf("outcome condition = %q, want %q", outcome, tc.wantCond)
			}
			if got := outcome.GetStatus(status); got != "True" {
				t.Fatalf("%s = %q, want True", outcome, got)
			}
			if got := outcome.GetReason(status); got != tc.wantReason {
				t.Fatalf("reason = %q, want %q", got, tc.wantReason)
			}
			if got := outcome.GetMessage(status); got != tc.wantMsg {
				t.Fatalf("message = %q, want %q", got, tc.wantMsg)
			}
		})
	}
}

// SetPhase keeps LastUpdated pointing at the moment the operation actually moved, which is what the
// operation controllers depend on to tell a status that has settled from one that is still moving.
func TestSetPhaseIsIdempotent(t *testing.T) {
	status := &OperationStatus{}
	status.SetPhase(OperationPhaseInProgress)

	moved := status.LastUpdated
	if moved.IsZero() {
		t.Fatal("the transition must be recorded")
	}

	status.SetPhase(OperationPhaseInProgress)
	if !status.LastUpdated.Equal(&moved) {
		t.Fatal("setting the phase it is already in must not move LastUpdated")
	}
}
