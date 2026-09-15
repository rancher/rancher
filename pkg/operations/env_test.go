package operations

import (
	"encoding/json"
	"testing"

	planapi "github.com/rancher/rancher/pkg/plan"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type testStep string

func testOperation(uid types.UID) metav1.Object {
	return &metav1.ObjectMeta{Name: "op", Namespace: "fleet-default", UID: uid}
}

func TestOperationEnv(t *testing.T) {
	t.Parallel()

	got := OperationEnv("etcd-snapshot-restore", testOperation("abc-123"), testStep("Preflight"))
	want := []string{
		"ETCD_SNAPSHOT_RESTORE_OPERATION_UID=abc-123",
		"ETCD_SNAPSHOT_RESTORE_STEP=Preflight",
	}
	if len(got) != len(want) {
		t.Fatalf("OperationEnv = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("OperationEnv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestOperationEnvIsStableForTheSameOperationAndStep(t *testing.T) {
	t.Parallel()

	// Reconciles of a single operation must produce identical plan content, otherwise the plan would
	// change every reconcile and re-trigger its instructions.
	first := OperationEnv("etcd-snapshot-save", testOperation("abc-123"), testStep("Save"))
	second := OperationEnv("etcd-snapshot-save", testOperation("abc-123"), testStep("Save"))
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("OperationEnv is not stable: %v then %v", first, second)
		}
	}
}

func TestOperationEnvDiffersPerOperationAndStep(t *testing.T) {
	t.Parallel()

	base := OperationEnv("etcd-snapshot-restore", testOperation("abc-123"), testStep("Preflight"))

	otherOp := OperationEnv("etcd-snapshot-restore", testOperation("def-456"), testStep("Preflight"))
	if base[0] == otherOp[0] {
		t.Errorf("a different operation must produce a different UID variable, both are %q", base[0])
	}

	otherStep := OperationEnv("etcd-snapshot-restore", testOperation("abc-123"), testStep("Shutdown"))
	if base[1] == otherStep[1] {
		t.Errorf("a different step must produce a different step variable, both are %q", base[1])
	}
}

func TestWithOperationEnv(t *testing.T) {
	t.Parallel()

	env := OperationEnv("encryption-key-rotation", testOperation("abc-123"), testStep("Rotate"))

	p := &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			{CommonInstruction: planapi.CommonInstruction{Name: "no-env"}},
			{CommonInstruction: planapi.CommonInstruction{Name: "existing-env", Env: []string{"RKE2_DATA_DIR=/var/lib/rancher/rke2"}}},
		},
		PeriodicInstructions: []planapi.PeriodicInstruction{
			{CommonInstruction: planapi.CommonInstruction{Name: "periodic"}},
		},
	}

	if got := WithOperationEnv(p, env); got != p {
		t.Error("WithOperationEnv should return the plan it was given so it can wrap an AssignPlan argument")
	}

	// Every instruction which can carry environment must be scoped, and pre-existing entries must
	// survive — the shutdown instruction relies on its data-directory variable.
	if got := p.OneTimeInstructions[0].Env; len(got) != 2 || got[0] != env[0] || got[1] != env[1] {
		t.Errorf("instruction without env = %v, want %v", got, env)
	}
	if got := p.OneTimeInstructions[1].Env; len(got) != 3 || got[0] != "RKE2_DATA_DIR=/var/lib/rancher/rke2" || got[1] != env[0] {
		t.Errorf("instruction with existing env = %v, want its own entry followed by %v", got, env)
	}
	if got := p.PeriodicInstructions[0].Env; len(got) != 2 || got[0] != env[0] {
		t.Errorf("periodic instruction env = %v, want %v", got, env)
	}
}

func TestWithOperationEnvChangesPlanBytes(t *testing.T) {
	t.Parallel()

	// This is the property the whole helper exists for: two operations running the same work must
	// serialize differently, so AssignPlan writes a new plan and the node runs it again.
	newPlan := func() *planapi.Plan {
		return &planapi.Plan{
			OneTimeInstructions: []planapi.OneTimeInstruction{
				{CommonInstruction: planapi.CommonInstruction{Name: "preflight", Command: "/bin/sh"}},
			},
		}
	}

	first, err := json.Marshal(WithOperationEnv(newPlan(), OperationEnv("etcd-snapshot-restore", testOperation("abc-123"), testStep("Preflight"))))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(WithOperationEnv(newPlan(), OperationEnv("etcd-snapshot-restore", testOperation("def-456"), testStep("Preflight"))))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) == string(second) {
		t.Fatalf("plans for two operations serialize identically, so the second would be reported as already applied: %s", first)
	}
}

func TestWithOperationEnvNilPlan(t *testing.T) {
	t.Parallel()

	// Callers which build a plan conditionally (a builder may skip and return nil) pass it through
	// unchecked.
	if got := WithOperationEnv(nil, []string{"A=b"}); got != nil {
		t.Errorf("WithOperationEnv(nil) = %v, want nil", got)
	}
}
