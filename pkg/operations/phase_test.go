package operations

import (
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsTerminated(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status opv1alpha1.OperationStatus
		want   bool
	}{
		{
			name:   "unset",
			status: opv1alpha1.OperationStatus{},
			want:   false,
		},
		{
			name: "terminal phase is not enough on its own",
			// An operation can sit in a terminal phase while its terminal phase hook is still
			// delegated to another controller, which is exactly the window in which deleting it
			// must cancel it rather than let it go quietly.
			status: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
			want:   false,
		},
		{
			name: "set",
			status: opv1alpha1.OperationStatus{
				Phase:        opv1alpha1.OperationPhaseSucceeded,
				TerminatedAt: metav1.Now(),
			},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, IsTerminated(&tc.status))
		})
	}
}

// TestSetTerminatedIsWriteOnce covers the property the operation controllers depend on: the terminal
// phase handlers call SetTerminated on every reconcile once they are done, and the controllers only
// poll, garbage-collect, and finish deletions when the resulting status is byte-identical to the
// previously stored one. A SetTerminated which moved the timestamp on each call would make every
// terminal operation look like it was still making progress, forever.
func TestSetTerminatedIsWriteOnce(t *testing.T) {
	t.Parallel()

	status := &opv1alpha1.OperationStatus{}

	status.SetTerminated()
	first := status.TerminatedAt
	assert.False(t, first.IsZero(), "SetTerminated must record a timestamp")

	// metav1.Time has second granularity, so a same-instant re-set would be invisible without
	// stepping the clock past a tick.
	status.TerminatedAt = metav1.NewTime(first.Add(-time.Hour))
	expected := status.TerminatedAt

	status.SetTerminated()
	assert.Equal(t, expected, status.TerminatedAt, "SetTerminated must not move an already-recorded timestamp")
}
