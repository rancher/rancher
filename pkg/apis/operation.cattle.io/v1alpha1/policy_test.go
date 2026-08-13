package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPreemptionPriority(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        int
	}{
		{name: "no annotations"},
		{name: "absent", annotations: map[string]string{"other": "100"}},
		{name: "empty", annotations: map[string]string{PreemptionPriorityAnnotation: ""}},
		{name: "not a number", annotations: map[string]string{PreemptionPriorityAnnotation: "high"}},
		{name: "zero", annotations: map[string]string{PreemptionPriorityAnnotation: "0"}},
		{name: "positive", annotations: map[string]string{PreemptionPriorityAnnotation: "100"}, want: 100},
		{name: "padded", annotations: map[string]string{PreemptionPriorityAnnotation: " 100 "}, want: 100},
		{name: "negative", annotations: map[string]string{PreemptionPriorityAnnotation: "-1"}, want: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := PreemptionPriority(test.annotations); got != test.want {
				t.Errorf("PreemptionPriority() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCanPreempt(t *testing.T) {
	tests := []struct {
		name   string
		mine   int
		theirs int
		want   bool
	}{
		{name: "neither may preempt", mine: 0, theirs: 0},
		{name: "a save may not preempt a restore", mine: 0, theirs: 100},
		{name: "a restore preempts a save", mine: 100, theirs: 0, want: true},
		{name: "a restore preempts another restore", mine: 100, theirs: 100, want: true},
		{name: "a lower priority may not preempt a higher one", mine: 50, theirs: 100},
		{name: "a higher priority preempts a lower one", mine: 100, theirs: 50, want: true},
		{name: "a negative priority never preempts", mine: -1, theirs: -1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanPreempt(test.mine, test.theirs); got != test.want {
				t.Errorf("CanPreempt(%d, %d) = %v, want %v", test.mine, test.theirs, got, test.want)
			}
		})
	}
}

func TestBoolAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{name: "absent"},
		{name: "true", annotations: map[string]string{ClearsRestoreRequiredAnnotation: "true"}, want: true},
		{name: "padded", annotations: map[string]string{ClearsRestoreRequiredAnnotation: " true "}, want: true},
		{name: "false", annotations: map[string]string{ClearsRestoreRequiredAnnotation: "false"}},
		{name: "nonsense", annotations: map[string]string{ClearsRestoreRequiredAnnotation: "yes please"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := BoolAnnotation(test.annotations, ClearsRestoreRequiredAnnotation); got != test.want {
				t.Errorf("BoolAnnotation() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestOperationPhaseIsTerminal(t *testing.T) {
	terminal := []OperationPhase{OperationPhaseSucceeded, OperationPhaseFailed, OperationPhaseCanceled}
	for _, phase := range terminal {
		if !phase.IsTerminal() {
			t.Errorf("%s should be terminal", phase)
		}
	}

	for _, phase := range []OperationPhase{"", OperationPhasePending, OperationPhaseInProgress} {
		if phase.IsTerminal() {
			t.Errorf("%q should not be terminal", phase)
		}
	}
}

func TestToOperation(t *testing.T) {
	save := &ETCDSnapshotSave{
		ObjectMeta: metav1.ObjectMeta{Namespace: "fleet-default", Name: "save"},
		Spec: ETCDSnapshotSaveSpec{
			OperationSpec: OperationSpec{
				ClusterRef: &corev1.ObjectReference{
					APIVersion: "management.cattle.io/v3",
					Kind:       "Cluster",
					Name:       "c-m-abcde",
				},
				Cancel: true,
			},
		},
		Status: ETCDSnapshotSaveStatus{
			OperationStatus: OperationStatus{Phase: OperationPhaseInProgress},
			Step:            ETCDSnapshotSaveStepSave,
		},
	}

	operation, err := ToOperation(save)
	if err != nil {
		t.Fatalf("ToOperation() error = %v", err)
	}

	// The typed object carries no TypeMeta, so the kind must come from the Go type.
	if operation.Kind != "ETCDSnapshotSave" {
		t.Errorf("Kind = %q, want ETCDSnapshotSave", operation.Kind)
	}
	if operation.Key() != "ETCDSnapshotSave/fleet-default/save" {
		t.Errorf("Key() = %q", operation.Key())
	}
	if operation.Status.Phase != OperationPhaseInProgress {
		t.Errorf("Phase = %q, want InProgress", operation.Status.Phase)
	}
	if !operation.Spec.Cancel {
		t.Error("Cancel should be true")
	}
	if operation.Spec.ClusterRef == nil || operation.Spec.ClusterRef.Name != "c-m-abcde" {
		t.Errorf("ClusterRef = %+v", operation.Spec.ClusterRef)
	}
}

func TestObjectRefKey(t *testing.T) {
	if got := ObjectRefKey("Cluster", "", "c-m-abcde"); got != "Cluster/c-m-abcde" {
		t.Errorf("ObjectRefKey() = %q", got)
	}
	if got := ObjectRefKey("ETCDSnapshotSave", "fleet-default", "save"); got != "ETCDSnapshotSave/fleet-default/save" {
		t.Errorf("ObjectRefKey() = %q", got)
	}
}
