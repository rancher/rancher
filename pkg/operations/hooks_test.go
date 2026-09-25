package operations

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestHasActiveLifecycleHook covers the predicate UpdateStatus reports an abandoned hook from: a
// false negative would let a terminated operation carrying a hook label look like it finished
// cleanly, hiding from whoever set the label that their delegate never got its turn. Recognizing
// the label is all it does — what gates garbage collection is termination, which is only recorded
// once the hook is satisfied or provably unsatisfiable. See Collectable.
func TestHasActiveLifecycleHook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{
			name: "nil labels",
			// obj with a nil GetLabels() must return false, matching the "no hook set" case
			// (many controllers construct scratch objects with no labels at all).
			labels: nil,
			want:   false,
		},
		{
			name:   "empty labels map",
			labels: map[string]string{},
			want:   false,
		},
		{
			name: "only unrelated labels",
			labels: map[string]string{
				"app":                     "rancher",
				"plan.cattle.io/owner":    "etcd-snapshot-save",
				"management.cattle.io/x":  "y",
				"rke.cattle.io/node-name": "node-1",
			},
			want: false,
		},
		{
			name: "pending phase hook",
			// Uses the actual exported prefix constant to catch drift if the string is ever
			// renamed.
			labels: map[string]string{opv1alpha1.PendingPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "in-progress phase hook",
			labels: map[string]string{opv1alpha1.InProgressPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "aborted phase hook",
			labels: map[string]string{opv1alpha1.AbortedPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "canceled phase hook",
			labels: map[string]string{opv1alpha1.CanceledPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "failed phase hook",
			labels: map[string]string{opv1alpha1.FailedPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "succeeded phase hook",
			labels: map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name: "step hook not exported by this package",
			// Step prefixes live in the operation-controller packages (e.g. save.step.hook.…,
			// rotate.step.hook.…). The predicate must still recognize them via the shared
			// marker so a controller-defined step hook keeps its op alive.
			labels: map[string]string{"save.step.hook.operation.cattle.io/my-hook": "delegate-a"},
			want:   true,
		},
		{
			name:   "arbitrary future step hook",
			labels: map[string]string{"future-op.step.hook.operation.cattle.io/x": "d"},
			want:   true,
		},
		{
			name: "hook label mixed with unrelated labels",
			labels: map[string]string{
				"app": "rancher",
				opv1alpha1.SucceededPhaseHookLabelPrefix + "test": "delegate-a",
				"rke.cattle.io/node-name":                         "node-1",
			},
			want: true,
		},
		{
			name: "hook-marker substring appears in label VALUE only",
			// The predicate checks label KEYS. A value containing the marker must NOT flip it.
			labels: map[string]string{"unrelated": ".hook.operation.cattle.io/nope"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &metav1.ObjectMeta{Labels: tt.labels}
			if got := HasActiveLifecycleHook(obj); got != tt.want {
				t.Fatalf("HasActiveLifecycleHook(labels=%v) = %v, want %v", tt.labels, got, tt.want)
			}
		})
	}

	// Nil metav1.Object argument — production controllers should never pass nil, but the
	// predicate must be defensive: any hypothetical caller receiving a nil (e.g. a client
	// returning nil on cache miss) must not panic.
	t.Run("nil object", func(t *testing.T) {
		if HasActiveLifecycleHook(nil) {
			t.Fatal("HasActiveLifecycleHook(nil) = true, want false")
		}
	})
}

// TestHasStepHookLabel covers the predicate handleInProgress uses to tell an intentional
// step-scoped delegation from a beacon it has genuinely lost. The empty-prefix case is
// load-bearing: a step with no hook prefix must not match every label on the operation.
func TestHasStepHookLabel(t *testing.T) {
	t.Parallel()

	const prefix = "rotate.step.hook.operation.cattle.io/"

	cases := []struct {
		name   string
		labels map[string]string
		prefix string
		want   bool
	}{
		{name: "no labels", prefix: prefix},
		{
			name:   "matching step hook",
			labels: map[string]string{prefix + "my-hook": "delegate-a"},
			prefix: prefix,
			want:   true,
		},
		{
			name:   "another step's hook",
			labels: map[string]string{"save.step.hook.operation.cattle.io/my-hook": "delegate-a"},
			prefix: prefix,
		},
		{
			// A phase hook is not a step hook: the two are asked about separately.
			name:   "phase hook",
			labels: map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "my-hook": "delegate-a"},
			prefix: prefix,
		},
		{
			name:   "empty prefix matches nothing",
			labels: map[string]string{prefix + "my-hook": "delegate-a"},
			prefix: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			obj := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Labels: tc.labels}}
			if got := HasStepHookLabel(obj, tc.prefix); got != tc.want {
				t.Fatalf("HasStepHookLabel(labels=%v, prefix=%q) = %v, want %v", tc.labels, tc.prefix, got, tc.want)
			}
		})
	}

	if HasStepHookLabel(nil, prefix) {
		t.Fatal("HasStepHookLabel(nil) = true, want false")
	}
}

// TestCollectable pins what garbage collection waits for: a terminal phase, the controller being
// done with the operation, and the TTL. Lifecycle hooks are deliberately not among them — the
// signature cannot even see them — because the paths that terminate while a hook label is still set
// are the ones where nothing will ever come back to clear it, so consulting the label would pin the
// operation for good. TestOnChange_MissingClusterAbandonsOwedHook covers that end to end in each
// controller.
func TestCollectable(t *testing.T) {
	t.Parallel()

	expired := func(phase opv1alpha1.OperationPhase, terminated bool) *opv1alpha1.OperationStatus {
		status := &opv1alpha1.OperationStatus{Phase: phase}
		if terminated {
			status.SetTerminated()
		}
		return status
	}

	tests := []struct {
		name   string
		spec   *opv1alpha1.OperationSpec
		status *opv1alpha1.OperationStatus
		want   bool
	}{
		{
			name:   "in flight",
			spec:   &opv1alpha1.OperationSpec{},
			status: expired(opv1alpha1.OperationPhaseInProgress, false),
		},
		{
			name: "terminal but the controller is not done",
			spec: &opv1alpha1.OperationSpec{},
			// The terminal phase hook is still delegated, so the beacon is held on the operation's
			// behalf and deleting it would strand the delegate.
			status: expired(opv1alpha1.OperationPhaseSucceeded, false),
		},
		{
			name:   "terminated but not expired",
			spec:   &opv1alpha1.OperationSpec{TTL: -1},
			status: expired(opv1alpha1.OperationPhaseSucceeded, true),
		},
		{
			name:   "terminated and expired",
			spec:   &opv1alpha1.OperationSpec{},
			status: expired(opv1alpha1.OperationPhaseSucceeded, true),
			want:   true,
		},
		{
			name:   "canceled, terminated and expired",
			spec:   &opv1alpha1.OperationSpec{},
			status: expired(opv1alpha1.OperationPhaseCanceled, true),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Collectable(tt.spec, tt.status); got != tt.want {
				t.Fatalf("Collectable = %v, want %v", got, tt.want)
			}
		})
	}
}
