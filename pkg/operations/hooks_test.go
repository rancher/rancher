package operations

import (
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		{
			// It names no delegate, so nothing was pushed onto the beacon for it.
			name:   "a step hook label with no delegate",
			labels: map[string]string{prefix + "my-hook": ""},
			prefix: prefix,
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

func TestHooksForPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		obj    metav1.Object
		prefix string
		want   []Hook
	}{
		{
			name:   "nil object",
			obj:    nil,
			prefix: "",
			want:   nil,
		},
		{
			name:   "empty",
			obj:    &metav1.ObjectMeta{},
			prefix: "",
			want:   nil,
		},
		{
			name:   "non-empty",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			prefix: "",
			want:   nil,
		},
		{
			name:   "different prefix",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			prefix: "baz",
			want:   []Hook{},
		},
		{
			name:   "matching prefix",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			prefix: "fo",
			want: []Hook{
				{
					Id:       "o",
					Delegate: "bar",
				},
			},
		},
		{
			name:   "exact prefix",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
			prefix: "foo",
			want:   []Hook{},
		},
		{
			name:   "on exact one non-exact prefix",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo": "bar", "foo2": "bar2"}},
			prefix: "foo",
			want: []Hook{
				{
					Id:       "2",
					Delegate: "bar2",
				},
			},
		},
		{
			name:   "multiple matching prefix",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo1": "bar1", "foo2": "bar2"}},
			prefix: "foo",
			want: []Hook{
				{
					Id:       "1",
					Delegate: "bar1",
				},
				{
					Id:       "2",
					Delegate: "bar2",
				},
			},
		},
		{
			name:   "a label naming no delegate is not a hook",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo/a": "", "foo/b": "bar"}},
			prefix: "foo/",
			want:   []Hook{{Id: "b", Delegate: "bar"}},
		},
		{
			name:   "multiple matching prefix in order",
			obj:    &metav1.ObjectMeta{Labels: map[string]string{"foo/a": "bar", "foo/b": "foo", "foo/c": "baz"}},
			prefix: "foo/",
			want: []Hook{
				{
					Id:       "a",
					Delegate: "bar",
				},
				{
					Id:       "b",
					Delegate: "foo",
				},
				{
					Id:       "c",
					Delegate: "baz",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := hooksForPrefix(tt.obj, tt.prefix)
			assert.Equal(t, tt.want, result)
		})
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

// A hook label with an empty value is a valid label, but names no delegate. It must not hide the hooks
// after it: the first hook is the one taken, so before empty values were skipped, a="" sorted ahead of
// b and the operation went on as if it had no hook at all.
func TestDelegateForHook_SkipsLabelsNamingNoDelegate(t *testing.T) {
	t.Parallel()

	prefix := opv1alpha1.SucceededPhaseHookLabelPrefix

	for name, tc := range map[string]struct {
		labels       map[string]string
		wantDelegate string
	}{
		"an empty hook ahead of a real one": {
			labels:       map[string]string{prefix + "a": "", prefix + "b": "follow-up"},
			wantDelegate: "follow-up",
		},
		"only an empty hook": {
			labels: map[string]string{prefix + "a": ""},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			obj := &metav1.ObjectMeta{Name: "op", Labels: tc.labels}
			beacons := &fakeBeaconClient{}
			beacon := &planv1alpha1.Beacon{Status: planv1alpha1.BeaconStatus{Active: true, Owner: "operation.cattle.io/ETCDSnapshotRestore/ns/op/uid"}}

			_, delegate := LifecycleHookDelegate(obj, prefix)
			assert.Equal(t, tc.wantDelegate, delegate)
			assert.Equal(t, tc.wantDelegate, TerminalHookDelegate(obj, opv1alpha1.OperationPhaseSucceeded))

			delegated, got, err := DelegateForHook(obj, beacon, beacons, prefix)
			assert.NoError(t, err)
			assert.Equal(t, tc.wantDelegate != "", delegated)
			if tc.wantDelegate == "" {
				assert.Empty(t, beacons.statusUpdates, "there is no hook, so the beacon is left alone")
				return
			}
			assert.Len(t, beacons.statusUpdates, 1)
			assert.Equal(t, []string{tc.wantDelegate}, got.Status.Delegates)
		})
	}
}
