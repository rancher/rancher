package system

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey(chartName string) desiredKey {
	return desiredKey{namespace: "cattle-system", chartName: chartName, releaseName: chartName}
}

// TestRetryQueueBackoffIsCumulative pins the queue's accounting: repeated failures for the same
// chart back off further each time rather than restarting from the base delay.
//
// The counterpart to this lives in runSync's dispatch, which deliberately leaves the entry in the
// queue while an install is in flight. Clearing it there would reset the count on every retry and
// hammer the API server every installRetryBaseDelay forever for a chart that cannot install. That
// half is enforced by the comment in dispatch, not by this test — the timings involved are too
// long to assert on directly.
func TestRetryQueueBackoffIsCumulative(t *testing.T) {
	q := newRetryQueue()
	key := testKey("system-upgrade-controller")
	now := time.Now()

	var (
		gotBackoffs []time.Duration
		gotAttempts []int
	)
	for range 4 {
		backoff, attempts := q.failed(key, map[string]interface{}{}, now)
		gotBackoffs = append(gotBackoffs, backoff)
		gotAttempts = append(gotAttempts, attempts)
	}

	assert.Equal(t, []int{1, 2, 3, 4}, gotAttempts, "the attempt count must accumulate across failures")
	assert.Equal(t, []time.Duration{
		installRetryBaseDelay,
		2 * installRetryBaseDelay,
		4 * installRetryBaseDelay,
		8 * installRetryBaseDelay,
	}, gotBackoffs)
}

// TestRetryQueueSucceededResetsBackoff covers the other half: once a chart installs, its history
// is dropped, so a later unrelated failure starts from the base delay again.
func TestRetryQueueSucceededResetsBackoff(t *testing.T) {
	q := newRetryQueue()
	key := testKey("fleet-crd")
	now := time.Now()

	q.failed(key, map[string]interface{}{}, now)
	q.failed(key, map[string]interface{}{}, now)
	q.succeeded(key)

	assert.Empty(t, q.due(now.Add(time.Hour)), "a chart that installed must not stay queued")

	backoff, attempts := q.failed(key, map[string]interface{}{}, now)
	assert.Equal(t, 1, attempts)
	assert.Equal(t, installRetryBaseDelay, backoff)
}

// TestRetryQueueParkDoesNotCountAnAttempt covers charts held behind the webhook. They were never
// tried, so parking must not consume backoff — otherwise waiting for the webhook would push a
// chart's retries out as if it had been failing.
func TestRetryQueueParkDoesNotCountAnAttempt(t *testing.T) {
	q := newRetryQueue()
	key := testKey("system-upgrade-controller")
	now := time.Now()

	d := desired{key: key, values: map[string]interface{}{}, takeOwnership: true}
	assert.True(t, q.park(d, now), "the first park reports the chart was newly queued")
	assert.False(t, q.park(d, now), "re-parking must report false so callers do not log repeatedly")

	require.Len(t, q.due(now), 1, "a parked chart is due immediately")

	backoff, attempts := q.failed(key, map[string]interface{}{}, now)
	assert.Equal(t, 1, attempts, "parking must not have counted as an attempt")
	assert.Equal(t, installRetryBaseDelay, backoff)
}

// TestRetryQueueParkPreservesAttempts guards the interaction between the two: a chart that has
// already failed and is then deferred behind the webhook keeps its backoff history.
func TestRetryQueueParkPreservesAttempts(t *testing.T) {
	q := newRetryQueue()
	key := testKey("system-upgrade-controller")
	now := time.Now()

	q.failed(key, map[string]interface{}{}, now)
	q.failed(key, map[string]interface{}{}, now)

	q.park(desired{key: key, values: map[string]interface{}{}, takeOwnership: true}, now)

	_, attempts := q.failed(key, map[string]interface{}{}, now)
	assert.Equal(t, 3, attempts)
}

func TestRetryQueueDue(t *testing.T) {
	q := newRetryQueue()
	now := time.Now()

	soon := testKey("rancher-webhook")
	later := testKey("system-upgrade-controller")

	q.park(desired{key: soon, values: map[string]interface{}{}}, now)
	q.failed(later, map[string]interface{}{}, now) // now + installRetryBaseDelay

	due := q.due(now)
	require.Len(t, due, 1)
	assert.Equal(t, soon, due[0].key, "only the entry whose wait has elapsed is due")

	due = q.due(now.Add(installRetryBaseDelay))
	assert.Len(t, due, 2, "at the deadline both entries are due")

	// due returns a snapshot, so callers can dispatch (and thereby re-queue) while iterating.
	due = q.due(now)
	q.park(desired{key: later, values: map[string]interface{}{}}, now)
	assert.Len(t, due, 1, "the returned slice must not alias the queue")
}
