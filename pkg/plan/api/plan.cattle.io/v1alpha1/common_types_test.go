package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestHasActiveLifecycleHook is the load-bearing regression test for the operation-controller
// TTL-delete guard: every controller's OnChange defers garbage collection while this predicate
// returns true, so a bug here would either leak operations indefinitely (false positive) or
// delete operations mid-hook and strand the delegate on the beacon (false negative). The
// controller-side usage is a single boolean `&& !HasActiveLifecycleHook(op)` in the delete
// condition, so this table-driven test on the predicate is the primary coverage.
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
			labels: map[string]string{PendingPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "in-progress phase hook",
			labels: map[string]string{InProgressPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "canceled phase hook",
			labels: map[string]string{CanceledPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "failed phase hook",
			labels: map[string]string{FailedPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name:   "succeeded phase hook",
			labels: map[string]string{SucceededPhaseHookLabelPrefix + "test": "delegate-a"},
			want:   true,
		},
		{
			name: "step hook not exported by this package",
			// Step prefixes live in the operation-controller packages (e.g. save.step.hook.…,
			// rotate.step.hook.…). The predicate must still recognise them via the shared
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
				SucceededPhaseHookLabelPrefix + "test": "delegate-a",
				"rke.cattle.io/node-name":              "node-1",
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
