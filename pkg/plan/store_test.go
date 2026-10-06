package plan

import (
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testWriter  = "operation.cattle.io/ETCDSnapshotSave/ns/save-1/uid-1"
	otherWriter = "operation.cattle.io/ETCDSnapshotSave/ns/save-0/uid-0"
)

// Helper function to build a mock secret pointer
func mockSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// fakeSecretUpdater records the secrets written through Update. Every other method of the generated
// client panics, so an unexpected call is loud rather than silently returning a zero value.
type fakeSecretUpdater struct {
	corecontrollers.SecretClient

	updates []*corev1.Secret
	err     error
}

func (f *fakeSecretUpdater) Update(secret *corev1.Secret) (*corev1.Secret, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.updates = append(f.updates, secret.DeepCopy())
	return secret, nil
}

// testPlan returns a plan to assign along with its serialized form, which is what AssignPlan compares
// against the plan already on the secret.
func testPlan(t *testing.T, command string) (*Plan, []byte) {
	t.Helper()

	p := &Plan{OneTimeInstructions: []OneTimeInstruction{{CommonInstruction: CommonInstruction{Name: "one", Command: command}}}}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	return p, raw
}

// attemptKey is a pseudo data key the secret helper below writes to PlanAttemptAnnotation instead,
// so a test case can describe the Store's attempt count alongside the data the agent records.
const attemptKey = "test-only/attempt"

// assigned returns a secret already holding raw as its plan, assigned by testWriter, carrying the
// given data.
func assigned(raw []byte, data map[string]string) *corev1.Secret {
	s := mockSecret("node-alpha")
	s.Data = map[string][]byte{PlanDataKey: raw}
	s.Annotations = map[string]string{PlanWriterAnnotation: testWriter}
	for k, v := range data {
		if k == attemptKey {
			s.Annotations[PlanAttemptAnnotation] = v
			continue
		}
		s.Data[k] = []byte(v)
	}
	return s
}

func TestPlanStatusMethods(t *testing.T) {
	tests := []struct {
		name    string
		status  PlanStatus
		success bool
		failure bool
	}{
		{name: "pending", status: PlanStatus{State: PlanStatePending}},
		{name: "in progress", status: PlanStatus{State: PlanStateInProgress}},
		{name: "in progress retrying", status: PlanStatus{State: PlanStateInProgress, Retrying: true}},
		{name: "paused", status: PlanStatus{State: PlanStatePaused}},
		{name: "succeeded awaiting probes", status: PlanStatus{State: PlanStateSucceeded}},
		{name: "succeeded and probes passed", status: PlanStatus{State: PlanStateSucceeded, ProbesPassed: true}, success: true},
		{name: "failed with a retry to come", status: PlanStatus{State: PlanStateFailed, Retrying: true}},
		{name: "failed", status: PlanStatus{State: PlanStateFailed}, failure: true},
		{name: "canceled", status: PlanStatus{State: PlanStateCanceled}, failure: true},
		// The agent treats a state it does not know as terminal, so nothing more will happen to it.
		{name: "unknown state", status: PlanStatus{State: "from-the-future"}, failure: true},
		{name: "no state", status: PlanStatus{}, failure: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.success, tt.status.Success(), "Success")
			assert.Equal(t, tt.failure, tt.status.Failure(), "Failure")
			assert.Equal(t, !tt.success && !tt.failure, tt.status.Waiting(), "Waiting")
		})
	}
}

func TestPlanStateIsActive(t *testing.T) {
	for state, active := range map[PlanState]bool{
		PlanStatePending:    true,
		PlanStateInProgress: true,
		PlanStatePaused:     true,
		PlanStateSucceeded:  false,
		PlanStateFailed:     false,
		PlanStateCanceled:   false,
		"":                  false,
	} {
		assert.Equal(t, active, state.IsActive(), "%q", state)
	}
}

func TestStoreCancelPlan(t *testing.T) {
	annotated := func(value string) *corev1.Secret {
		s := mockSecret("node-alpha")
		s.Annotations = map[string]string{PlanCanceledAnnotation: value, PlanWriterAnnotation: otherWriter}
		return s
	}

	t.Run("annotates a plan that is not already canceled, on behalf of the writer", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, updated, err := NewStore(client).CancelPlan(mockSecret("node-alpha"), testWriter)
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, "true", updated.Annotations[PlanCanceledAnnotation])
		require.Len(t, client.updates, 1)
		assert.Equal(t, "true", client.updates[0].Annotations[PlanCanceledAnnotation])
		assert.Equal(t, testWriter, client.updates[0].Annotations[PlanWriterAnnotation],
			"the webhook checks the cancellation against the beacon like any other write")
	})

	// A caller which cannot tell whether an earlier attempt landed calls this again, so a second
	// call must not issue a pointless write.
	t.Run("is idempotent", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, updated, err := NewStore(client).CancelPlan(annotated("true"), testWriter)
		require.NoError(t, err)
		assert.False(t, written)
		assert.Empty(t, client.updates)
		assert.Equal(t, "true", updated.Annotations[PlanCanceledAnnotation])
	})

	// "false" is the annotation's other valid value and means the plan is not canceled, so it is
	// overwritten rather than read as "already handled".
	t.Run("overwrites an explicit false", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, _, err := NewStore(client).CancelPlan(annotated("false"), testWriter)
		require.NoError(t, err)
		assert.True(t, written)
		require.Len(t, client.updates, 1)
		assert.Equal(t, testWriter, client.updates[0].Annotations[PlanWriterAnnotation])
	})

	t.Run("returns the secret it was given when the write fails", func(t *testing.T) {
		client := &fakeSecretUpdater{err: errors.New("boom")}
		secret := mockSecret("node-alpha")

		written, returned, err := NewStore(client).CancelPlan(secret, testWriter)
		require.Error(t, err)
		assert.False(t, written)
		assert.Same(t, secret, returned, "the caller must be left with a usable secret")
		assert.Empty(t, secret.Annotations, "the secret it was given must not be mutated")
	})

	t.Run("a nil secret is a no-op", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, returned, err := NewStore(client).CancelPlan(nil, testWriter)
		require.NoError(t, err)
		assert.False(t, written)
		assert.Nil(t, returned)
		assert.Empty(t, client.updates)
	})
}

func TestStoreAssignPlanNewAssignment(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = time.Now })

	plan, raw := testPlan(t, "true")

	// assertFresh asserts that written holds plan as a fresh assignment by testWriter, with nothing
	// carried over from whatever the secret held before.
	assertFresh := func(t *testing.T, written *corev1.Secret) {
		t.Helper()
		assert.Equal(t, raw, written.Data[PlanDataKey])
		assert.Equal(t, string(PlanStatePending), string(written.Data[PlanStateKey]))
		assert.Equal(t, testWriter, written.Annotations[PlanWriterAnnotation])
		assert.Equal(t, "1", written.Annotations[PlanAttemptAnnotation])
		assert.Equal(t, fixed.Format(time.RFC3339), written.Annotations[PlanLastUpdatedAnnotation])
		assert.Equal(t, "", written.Annotations[PlanProbesPassedAnnotation])
		assert.NotContains(t, written.Annotations, PlanCanceledAnnotation)
		assert.NotContains(t, written.Data, probeStatusesKey)
		assert.Contains(t, written.Data, PlanCheckpointKey, "the checkpoint is cleared with an empty value, as the agent clears it")
		assert.Empty(t, written.Data[PlanCheckpointKey])
	}

	// previous is a secret left behind by an earlier assignment, with everything the agent and the
	// Store record about it.
	previous := func(content []byte, writer string, state PlanState) *corev1.Secret {
		s := mockSecret("node-alpha")
		s.Data = map[string][]byte{
			PlanDataKey:       content,
			PlanStateKey:      []byte(state),
			probeStatusesKey:  []byte(`{"p":{"healthy":true}}`),
			PlanCheckpointKey: []byte(`{"checksum":"x","completedInstructions":2,"paused":true}`),
			"failure-count":   []byte("3"),
			"applied-output":  []byte("left alone"),
		}
		s.Annotations = map[string]string{
			PlanWriterAnnotation:       writer,
			PlanAttemptAnnotation:      "4",
			PlanCanceledAnnotation:     "true",
			PlanProbesPassedAnnotation: "yes",
		}
		return s
	}

	for _, state := range []PlanState{PlanStateSucceeded, PlanStateFailed, PlanStateCanceled, PlanStateInProgress, PlanStatePaused, PlanStatePending} {
		t.Run("new content after "+string(state), func(t *testing.T) {
			client := &fakeSecretUpdater{}
			status, err := NewStore(client).AssignPlan(previous([]byte("previous"), testWriter, state), plan, testWriter, 1, 1)
			require.NoError(t, err)
			assert.Equal(t, &PlanStatus{Secret: status.Secret, State: PlanStatePending}, status)
			require.Len(t, client.updates, 1)
			assertFresh(t, client.updates[0])
		})

		// The next operation to run on a cluster can compute exactly the plan the previous one left on
		// the secret. It is still a new assignment, and is run again rather than reported on the
		// previous one's outcome.
		t.Run("identical content from another writer after "+string(state), func(t *testing.T) {
			client := &fakeSecretUpdater{}
			status, err := NewStore(client).AssignPlan(previous(raw, otherWriter, state), plan, testWriter, 1, 1)
			require.NoError(t, err)
			assert.Equal(t, &PlanStatus{Secret: status.Secret, State: PlanStatePending}, status)
			require.Len(t, client.updates, 1)
			assertFresh(t, client.updates[0])
			assert.Equal(t, "left alone", string(client.updates[0].Data["applied-output"]), "the agent's records are its own")
			assert.Equal(t, "3", string(client.updates[0].Data["failure-count"]), "the agent's records are its own")
		})
	}

	// A plan assigned before writers were recorded has none, so its next reassignment is fresh.
	t.Run("identical content assigned before writers were recorded", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := previous(raw, "", PlanStateSucceeded)
		delete(secret.Annotations, PlanWriterAnnotation)
		_, err := NewStore(client).AssignPlan(secret, plan, testWriter, 1, 1)
		require.NoError(t, err)
		require.Len(t, client.updates, 1)
		assertFresh(t, client.updates[0])
	})

	t.Run("initializes a secret with no data or annotations", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(mockSecret("node-alpha"), plan, testWriter, 0, 0)
		require.NoError(t, err)
		assert.Equal(t, PlanStatePending, status.State)
		require.Len(t, client.updates, 1)
		assertFresh(t, client.updates[0])
	})

	t.Run("writes the failure limits", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		_, err := NewStore(client).AssignPlan(mockSecret("node-alpha"), plan, testWriter, 5, 3)
		require.NoError(t, err)
		require.Len(t, client.updates, 1)
		assert.Equal(t, "5", string(client.updates[0].Data[maxFailuresKey]))
		assert.Equal(t, "3", string(client.updates[0].Data[failureThresholdKey]))
	})

	t.Run("unset failure limits remove the previous plan's", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := assigned([]byte("previous"), map[string]string{maxFailuresKey: "5", failureThresholdKey: "5"})
		_, err := NewStore(client).AssignPlan(secret, plan, testWriter, 0, 0)
		require.NoError(t, err)
		require.Len(t, client.updates, 1)
		assert.NotContains(t, client.updates[0].Data, maxFailuresKey)
		assert.NotContains(t, client.updates[0].Data, failureThresholdKey)
	})

	t.Run("unlimited failure limits are written", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		_, err := NewStore(client).AssignPlan(mockSecret("node-alpha"), plan, testWriter, -1, -1)
		require.NoError(t, err)
		require.Len(t, client.updates, 1)
		assert.Equal(t, "-1", string(client.updates[0].Data[maxFailuresKey]))
		assert.Equal(t, "-1", string(client.updates[0].Data[failureThresholdKey]))
	})

	t.Run("does not mutate the secret it was given", func(t *testing.T) {
		secret := previous([]byte("previous"), otherWriter, PlanStateSucceeded)
		original := secret.DeepCopy()
		_, err := NewStore(&fakeSecretUpdater{}).AssignPlan(secret, plan, testWriter, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, original, secret)
	})

	t.Run("returns the update error", func(t *testing.T) {
		status, err := NewStore(&fakeSecretUpdater{err: errors.New("boom")}).AssignPlan(mockSecret("node-alpha"), plan, testWriter, 1, 1)
		assert.Error(t, err)
		assert.Nil(t, status)
	})
}

// Assigning the plan the secret already holds, from the writer that assigned it, is how every
// reconcile after the first checks on it: nothing is written unless a failed plan is retried, and
// the status reports what the agent recorded.
func TestStoreAssignPlanReportsState(t *testing.T) {
	plan, raw := testPlan(t, "true")
	healthyProbes := `{"probe":{"healthy":true}}`

	tests := []struct {
		name         string
		data         map[string]string
		probesPassed bool
		expected     PlanStatus
	}{
		{name: "pending", data: map[string]string{PlanStateKey: string(PlanStatePending)}, expected: PlanStatus{State: PlanStatePending}},
		{name: "pending retry", data: map[string]string{PlanStateKey: string(PlanStatePending), attemptKey: "2"}, expected: PlanStatus{State: PlanStatePending, Retrying: true}},
		{name: "in progress", data: map[string]string{PlanStateKey: string(PlanStateInProgress)}, expected: PlanStatus{State: PlanStateInProgress}},
		{name: "in progress on its first attempt", data: map[string]string{PlanStateKey: string(PlanStateInProgress), attemptKey: "1"}, expected: PlanStatus{State: PlanStateInProgress}},
		{name: "in progress retry", data: map[string]string{PlanStateKey: string(PlanStateInProgress), attemptKey: "2"}, expected: PlanStatus{State: PlanStateInProgress, Retrying: true}},
		{
			// The agent keeps counting failures across assignments; they say nothing about this one.
			name:     "in progress with failures left by an earlier assignment",
			data:     map[string]string{PlanStateKey: string(PlanStateInProgress), "failure-count": "3", "failed-checksum": PlanHash(raw)},
			expected: PlanStatus{State: PlanStateInProgress},
		},
		{name: "paused", data: map[string]string{PlanStateKey: string(PlanStatePaused)}, expected: PlanStatus{State: PlanStatePaused}},
		{name: "succeeded awaiting probes", data: map[string]string{PlanStateKey: string(PlanStateSucceeded)}, expected: PlanStatus{State: PlanStateSucceeded}},
		{
			name:         "succeeded and probes passed",
			data:         map[string]string{PlanStateKey: string(PlanStateSucceeded), probeStatusesKey: healthyProbes},
			probesPassed: true,
			expected:     PlanStatus{State: PlanStateSucceeded, ProbesPassed: true},
		},
		{
			name:         "succeeded with an unhealthy probe",
			data:         map[string]string{PlanStateKey: string(PlanStateSucceeded), probeStatusesKey: `{"a":{"healthy":true},"b":{}}`},
			probesPassed: true,
			expected:     PlanStatus{State: PlanStateSucceeded},
		},
		{
			name:     "succeeded with healthy probes not yet recorded as passed",
			data:     map[string]string{PlanStateKey: string(PlanStateSucceeded), probeStatusesKey: healthyProbes},
			expected: PlanStatus{State: PlanStateSucceeded},
		},
		{
			name:         "succeeded, recorded as passed, but no probe statuses",
			data:         map[string]string{PlanStateKey: string(PlanStateSucceeded)},
			probesPassed: true,
			expected:     PlanStatus{State: PlanStateSucceeded},
		},
		{name: "failed", data: map[string]string{PlanStateKey: string(PlanStateFailed), failureThresholdKey: "1"}, expected: PlanStatus{State: PlanStateFailed}},
		{name: "failed without a threshold set", data: map[string]string{PlanStateKey: string(PlanStateFailed)}, expected: PlanStatus{State: PlanStateFailed}},
		{name: "canceled", data: map[string]string{PlanStateKey: string(PlanStateCanceled)}, expected: PlanStatus{State: PlanStateCanceled}},
		{name: "unknown state", data: map[string]string{PlanStateKey: "from-the-future"}, expected: PlanStatus{State: "from-the-future"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := assigned(raw, tt.data)
			if tt.probesPassed {
				secret.Annotations[PlanProbesPassedAnnotation] = "yes"
			}
			client := &fakeSecretUpdater{}

			status, err := NewStore(client).AssignPlan(secret, plan, testWriter, 1, 1)
			require.NoError(t, err)
			tt.expected.Secret = status.Secret
			assert.Equal(t, &tt.expected, status)
			assert.Empty(t, client.updates)

			// Status reports the same without being able to write at all.
			read, err := Status(secret)
			require.NoError(t, err)
			tt.expected.Secret = secret
			assert.Equal(t, &tt.expected, read)
		})
	}
}

func TestStoreAssignPlanRetry(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = time.Now })

	plan, raw := testPlan(t, "true")

	failed := func(data map[string]string) *corev1.Secret {
		all := map[string]string{PlanStateKey: string(PlanStateFailed), maxFailuresKey: "3", failureThresholdKey: "3"}
		for k, v := range data {
			all[k] = v
		}
		return assigned(raw, all)
	}

	// retried assigns the plan over secret and reports whether it was retried, asserting that a plan
	// with a retry to come is never reported as a failure either way.
	retried := func(t *testing.T, secret *corev1.Secret) (bool, []*corev1.Secret) {
		t.Helper()
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(secret, plan, testWriter, 3, 3)
		require.NoError(t, err)
		assert.True(t, status.Retrying)
		assert.False(t, status.Failure())
		assert.True(t, status.Waiting())
		return len(client.updates) == 1 && status.State == PlanStatePending, client.updates
	}

	t.Run("resets the plan to pending as the next attempt", func(t *testing.T) {
		secret := failed(map[string]string{attemptKey: "1", probeStatusesKey: `{"p":{"healthy":true}}`})
		secret.Annotations[PlanProbesPassedAnnotation] = "yes"

		ok, updates := retried(t, secret)
		require.True(t, ok)
		written := updates[0]
		assert.Equal(t, raw, written.Data[PlanDataKey], "a retry must not change the plan")
		assert.Equal(t, string(PlanStatePending), string(written.Data[PlanStateKey]))
		assert.Equal(t, "2", written.Annotations[PlanAttemptAnnotation])
		assert.Equal(t, testWriter, written.Annotations[PlanWriterAnnotation])
		assert.NotContains(t, written.Data, probeStatusesKey)
		assert.Equal(t, "", written.Annotations[PlanProbesPassedAnnotation])
		assert.Equal(t, fixed.Format(time.RFC3339), written.Annotations[PlanLastUpdatedAnnotation])
	})

	t.Run("a plan assigned before attempts were counted is on its first", func(t *testing.T) {
		ok, updates := retried(t, failed(nil))
		require.True(t, ok)
		assert.Equal(t, "2", updates[0].Annotations[PlanAttemptAnnotation])
	})

	t.Run("retries with unlimited attempts", func(t *testing.T) {
		ok, _ := retried(t, failed(map[string]string{attemptKey: "40", maxFailuresKey: "-1", failureThresholdKey: "-1"}))
		assert.True(t, ok)
	})

	t.Run("retries when max-failures is unset", func(t *testing.T) {
		secret := failed(map[string]string{attemptKey: "2"})
		delete(secret.Data, maxFailuresKey)
		ok, _ := retried(t, secret)
		assert.True(t, ok)
	})

	t.Run("retries when last-apply-time is unparsable", func(t *testing.T) {
		ok, _ := retried(t, failed(map[string]string{lastApplyTimeKey: "yesterday"}))
		assert.True(t, ok)
	})

	t.Run("waits out the cooldown", func(t *testing.T) {
		ok, _ := retried(t, failed(map[string]string{lastApplyTimeKey: fixed.Add(-10 * time.Second).Format(time.UnixDate)}))
		assert.False(t, ok)
	})

	t.Run("retries exactly once the cooldown has passed", func(t *testing.T) {
		ok, _ := retried(t, failed(map[string]string{lastApplyTimeKey: fixed.Add(-failedPlanRetryCooldown).Format(time.UnixDate)}))
		assert.True(t, ok)
	})

	// The agent increments failure-count on every failure and resets it only on success, so it
	// still holds the failures of earlier assignments, matching failed-checksum or not.
	for _, failedChecksum := range []string{PlanHash(raw), "other"} {
		t.Run("ignores the agent's failure count, failed-checksum "+failedChecksum, func(t *testing.T) {
			ok, _ := retried(t, failed(map[string]string{"failed-checksum": failedChecksum, "failure-count": "9"}))
			assert.True(t, ok)
		})
	}

	notRetried := func(t *testing.T, secret *corev1.Secret, maxFailures, failureThreshold int) *PlanStatus {
		t.Helper()
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(secret, plan, testWriter, maxFailures, failureThreshold)
		require.NoError(t, err)
		assert.Empty(t, client.updates)
		assert.False(t, status.Retrying)
		assert.True(t, status.Failure())
		return status
	}

	t.Run("does not retry a plan that reached its threshold", func(t *testing.T) {
		notRetried(t, failed(map[string]string{attemptKey: "3", maxFailuresKey: "5"}), 5, 3)
	})

	t.Run("does not retry the first failure with a threshold of one", func(t *testing.T) {
		notRetried(t, failed(map[string]string{failureThresholdKey: "1"}), 1, 1)
	})

	// The threshold being unlimited allows a plan to run out of attempts short of it. Nothing will
	// happen to it after that, so it is a failure rather than something to wait on forever.
	t.Run("does not retry a plan out of attempts", func(t *testing.T) {
		notRetried(t, failed(map[string]string{attemptKey: "3", failureThresholdKey: "-1"}), 3, -1)
	})

	t.Run("does not retry a canceled plan", func(t *testing.T) {
		secret := failed(map[string]string{attemptKey: "1"})
		secret.Annotations[PlanCanceledAnnotation] = "true"
		notRetried(t, secret, 3, 3)
	})

	t.Run("returns the update error", func(t *testing.T) {
		status, err := NewStore(&fakeSecretUpdater{err: errors.New("boom")}).AssignPlan(failed(nil), plan, testWriter, 3, 3)
		assert.Error(t, err)
		assert.Nil(t, status)
	})

	// Runs the whole retry budget the way the agent and the Store take turns at the secret, on top of
	// failures an earlier assignment left in the agent's count: each failure is retried as the next
	// attempt, until the threshold is reached on the last.
	t.Run("retries until the threshold", func(t *testing.T) {
		store := NewStore(&fakeSecretUpdater{})
		secret := assigned([]byte("previous"), map[string]string{"failure-count": "4"})
		status, err := store.AssignPlan(secret, plan, testWriter, 3, 3)
		require.NoError(t, err)
		require.Equal(t, "1", status.Secret.Annotations[PlanAttemptAnnotation])

		for attempt := 1; attempt <= 3; attempt++ {
			// The agent runs the attempt and fails it.
			secret := status.Secret.DeepCopy()
			secret.Data[PlanStateKey] = []byte(PlanStateFailed)
			secret.Data["failure-count"] = []byte(strconv.Itoa(4 + attempt))

			status, err = store.AssignPlan(secret, plan, testWriter, 3, 3)
			require.NoError(t, err)
			if attempt < 3 {
				assert.Equal(t, PlanStatePending, status.State, "attempt %d should be retried", attempt)
				assert.True(t, status.Retrying)
				assert.Equal(t, strconv.Itoa(attempt+1), status.Secret.Annotations[PlanAttemptAnnotation])
				continue
			}
			assert.Equal(t, &PlanStatus{Secret: secret, State: PlanStateFailed}, status, "the last attempt reaches the threshold")
		}
	})

	// The agent records a retry it has picked up as in-progress, and once it succeeds the plan's
	// earlier failures no longer matter.
	t.Run("a retry runs to success", func(t *testing.T) {
		secret := assigned(raw, map[string]string{PlanStateKey: string(PlanStateInProgress), attemptKey: "2"})
		status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(secret, plan, testWriter, 3, 3)
		require.NoError(t, err)
		assert.Equal(t, &PlanStatus{Secret: status.Secret, State: PlanStateInProgress, Retrying: true}, status)

		secret.Data[PlanStateKey] = []byte(PlanStateSucceeded)
		secret.Data[probeStatusesKey] = []byte(`{"p":{"healthy":true}}`)
		secret.Annotations[PlanProbesPassedAnnotation] = "yes"
		status, err = NewStore(&fakeSecretUpdater{}).AssignPlan(secret, plan, testWriter, 3, 3)
		require.NoError(t, err)
		assert.True(t, status.Success())
	})
}

func TestStoreAssignPlanMalformedData(t *testing.T) {
	plan, raw := testPlan(t, "true")

	tests := []struct {
		name string
		data map[string]string
	}{
		{name: "probe statuses", data: map[string]string{PlanStateKey: string(PlanStateSucceeded), probeStatusesKey: "{"}},
		{name: "attempt while in progress", data: map[string]string{PlanStateKey: string(PlanStateInProgress), attemptKey: "x"}},
		{name: "attempt of a failed plan", data: map[string]string{PlanStateKey: string(PlanStateFailed), attemptKey: "x"}},
		{name: "failure threshold", data: map[string]string{PlanStateKey: string(PlanStateFailed), failureThresholdKey: "x"}},
		{name: "max failures", data: map[string]string{PlanStateKey: string(PlanStateFailed), failureThresholdKey: "3", maxFailuresKey: "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := assigned(raw, tt.data)
			secret.Annotations[PlanProbesPassedAnnotation] = "yes"
			client := &fakeSecretUpdater{}
			status, err := NewStore(client).AssignPlan(secret, plan, testWriter, 3, 3)
			assert.Error(t, err)
			assert.Nil(t, status)
			assert.Empty(t, client.updates)

			_, err = Status(secret)
			assert.Error(t, err)
		})
	}

	// The agent's failure records are not read, so they cannot fail the plan's status.
	for _, state := range []PlanState{PlanStateInProgress, PlanStateFailed} {
		t.Run("agent failure count while "+string(state), func(t *testing.T) {
			_, err := NewStore(&fakeSecretUpdater{}).AssignPlan(assigned(raw, map[string]string{
				PlanStateKey: string(state), "failed-checksum": PlanHash(raw), "failure-count": "x",
			}), plan, testWriter, 3, 3)
			require.NoError(t, err)
		})
	}

	// Malformed data on the plan being replaced does not matter: it is never read.
	t.Run("malformed data of the previous plan", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		_, err := NewStore(client).AssignPlan(assigned([]byte("previous"), map[string]string{
			PlanStateKey: string(PlanStateFailed), attemptKey: "x", failureThresholdKey: "x",
		}), plan, testWriter, 3, 3)
		require.NoError(t, err)
		assert.Len(t, client.updates, 1)
	})
}

func TestBeaconRefForSecret(t *testing.T) {
	secret := func(namespace string, labels map[string]string) *corev1.Secret {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "machine-plan", Namespace: namespace, Labels: labels}}
	}

	tests := []struct {
		name          string
		secret        *corev1.Secret
		wantNamespace string
		wantName      string
		wantOK        bool
	}{
		// An imported cluster's secrets and beacon are in the namespace named after the cluster.
		{name: "imported", secret: secret("c-m-abc", map[string]string{planv1alpha1.ClusterLifecycleNameLabel: "c-m-abc"}), wantNamespace: "c-m-abc", wantName: "c-m-abc", wantOK: true},
		// A CAPR or CAPRKE2 cluster's are in its CAPI cluster's namespace, under the cluster's name.
		{name: "capr", secret: secret("fleet-default", map[string]string{planv1alpha1.ClusterLifecycleNameLabel: "prod"}), wantNamespace: "fleet-default", wantName: "prod", wantOK: true},
		// The lifecycle label is the one read; the label the collector selects by is not consulted.
		{name: "only rke.cattle.io/cluster-name", secret: secret("fleet-default", map[string]string{labelClusterName: "prod"})},
		{name: "no cluster name", secret: secret("fleet-default", map[string]string{"other": "label"})},
		{name: "empty cluster name", secret: secret("fleet-default", map[string]string{planv1alpha1.ClusterLifecycleNameLabel: ""})},
		{name: "no labels", secret: secret("fleet-default", nil)},
		{name: "nil secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, name, ok := BeaconRefForSecret(tt.secret)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantNamespace, namespace)
			assert.Equal(t, tt.wantName, name)
		})
	}
}
