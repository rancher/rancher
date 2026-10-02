package plan

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to build a mock secret pointer
func mockSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name     string
		results  []PlanStatus
		expected string
	}{
		{
			name:     "Empty results slice",
			results:  []PlanStatus{},
			expected: "",
		},
		{
			name: "Nil secret items are skipped safely",
			results: []PlanStatus{
				{Secret: nil, Pending: true},
			},
			expected: "",
		},
		{
			name: "Single node waiting for plan applied",
			results: []PlanStatus{
				{Secret: mockSecret("node-alpha"), InProgress: true},
			},
			expected: "waiting for plan applied for node-alpha",
		},
		{
			name: "Two nodes waiting for plan picked up (Verifies exact suffix '1 other node')",
			results: []PlanStatus{
				// Out of order to test lexicographical sorting picks node-alpha as primary
				{Secret: mockSecret("node-beta"), Pending: true},
				{Secret: mockSecret("node-alpha"), Pending: true},
			},
			expected: "waiting for plan to be picked up for node-alpha & 1 other node",
		},
		{
			name: "Four nodes waiting for probes (Verifies plural scaling suffix)",
			results: []PlanStatus{
				{Secret: mockSecret("node-d"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-b"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-a"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-c"), Applied: true, ProbesPassed: false},
			},
			expected: "waiting for probes for node-a & 3 other nodes",
		},
		{
			name: "Strictly failed or completely successful nodes are excluded",
			results: []PlanStatus{
				{Secret: mockSecret("node-good"), Applied: true, ProbesPassed: true},
				{Secret: mockSecret("node-dead"), Failed: true},
			},
			expected: "",
		},
		{
			name: "Mixed messages with priority ordering (Failing -> Pending -> InProgress -> Probes)",
			results: []PlanStatus{
				{Secret: mockSecret("node-probes"), Applied: true, ProbesPassed: false},
				{Secret: mockSecret("node-failing"), Failing: true, Failed: false},
				{Secret: mockSecret("node-progress"), InProgress: true},
				{Secret: mockSecret("node-pending"), Pending: true},
			},
			expected: "failing plan for node-failing, waiting for plan to be picked up for node-pending, waiting for plan applied for node-progress, waiting for probes for node-probes",
		},
		{
			name: "Mixed messages with duplicate nodes per tier",
			results: []PlanStatus{
				{Secret: mockSecret("node-p1"), Pending: true},
				{Secret: mockSecret("node-p2"), Pending: true},
				{Secret: mockSecret("node-f1"), Failing: true, Failed: false},
				{Secret: mockSecret("node-f2"), Failing: true, Failed: false},
				{Secret: mockSecret("node-f3"), Failing: true, Failed: false},
			},
			expected: "failing plan for node-f1 & 2 other nodes, waiting for plan to be picked up for node-p1 & 1 other node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Message(tt.results)
			if actual != tt.expected {
				t.Errorf("\nExpected: %q\nGot:      %q", tt.expected, actual)
			}
		})
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

func TestStoreCancelPlan(t *testing.T) {
	annotated := func(value string) *corev1.Secret {
		s := mockSecret("node-alpha")
		s.Annotations = map[string]string{PlanCanceledAnnotation: value}
		return s
	}

	t.Run("annotates a plan that is not already canceled", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, updated, err := NewStore(client).CancelPlan(mockSecret("node-alpha"))
		require.NoError(t, err)
		assert.True(t, written)
		assert.Equal(t, "true", updated.Annotations[PlanCanceledAnnotation])
		require.Len(t, client.updates, 1)
		assert.Equal(t, "true", client.updates[0].Annotations[PlanCanceledAnnotation])
	})

	// A caller which cannot tell whether an earlier attempt landed calls this again, so a second
	// call must not issue a pointless write.
	t.Run("is idempotent", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, updated, err := NewStore(client).CancelPlan(annotated("true"))
		require.NoError(t, err)
		assert.False(t, written)
		assert.Empty(t, client.updates)
		assert.Equal(t, "true", updated.Annotations[PlanCanceledAnnotation])
	})

	// "false" is the annotation's other valid value and means the plan is not canceled, so it is
	// overwritten rather than read as "already handled".
	t.Run("overwrites an explicit false", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, _, err := NewStore(client).CancelPlan(annotated("false"))
		require.NoError(t, err)
		assert.True(t, written)
		require.Len(t, client.updates, 1)
	})

	t.Run("returns the secret it was given when the write fails", func(t *testing.T) {
		client := &fakeSecretUpdater{err: errors.New("boom")}
		secret := mockSecret("node-alpha")

		written, returned, err := NewStore(client).CancelPlan(secret)
		require.Error(t, err)
		assert.False(t, written)
		assert.Same(t, secret, returned, "the caller must be left with a usable secret")
		assert.Empty(t, secret.Annotations, "the secret it was given must not be mutated")
	})

	t.Run("a nil secret is a no-op", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		written, returned, err := NewStore(client).CancelPlan(nil)
		require.NoError(t, err)
		assert.False(t, written)
		assert.Nil(t, returned)
		assert.Empty(t, client.updates)
	})
}

func TestStoreAssignPlan(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = time.Now })

	assigned := &Plan{OneTimeInstructions: []OneTimeInstruction{{CommonInstruction: CommonInstruction{Name: "one", Command: "true"}}}}
	raw, err := json.Marshal(assigned)
	require.NoError(t, err)
	checksum := PlanHash(raw)

	healthyProbes := []byte(`{"probe":{"healthy":true}}`)

	// withPlan returns a secret already carrying the assigned plan, with data merged in.
	withPlan := func(data map[string]string, annotations map[string]string) *corev1.Secret {
		s := mockSecret("node-alpha")
		s.Data = map[string][]byte{PlanDataKey: raw}
		for k, v := range data {
			s.Data[k] = []byte(v)
		}
		s.Annotations = annotations
		return s
	}

	t.Run("new content is written as pending", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := mockSecret("node-alpha")
		secret.Data = map[string][]byte{
			PlanStateKey:     []byte(PlanStateSucceeded),
			probeStatusesKey: healthyProbes,
		}
		secret.Annotations = map[string]string{PlanCanceledAnnotation: "true"}

		status, err := NewStore(client).AssignPlan(secret, assigned, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, &PlanStatus{Secret: status.Secret, Pending: true}, status)
		require.Len(t, client.updates, 1)
		written := client.updates[0]
		assert.Equal(t, string(PlanStatePending), string(written.Data[PlanStateKey]))
		assert.Equal(t, raw, written.Data[PlanDataKey])
		assert.Equal(t, "1", string(written.Data[maxFailuresKey]))
		assert.Equal(t, "1", string(written.Data[failureThresholdKey]))
		assert.NotContains(t, written.Data, probeStatusesKey)
		assert.NotContains(t, written.Annotations, PlanCanceledAnnotation)
		assert.Equal(t, "", written.Annotations[PlanProbesPassedAnnotation])
	})

	tests := []struct {
		name        string
		data        map[string]string
		annotations map[string]string
		expected    PlanStatus
		// retried is whether plan-state is expected to have been reset to pending.
		retried bool
	}{
		{
			name:     "pending and not yet picked up",
			data:     map[string]string{PlanStateKey: string(PlanStatePending)},
			expected: PlanStatus{Pending: true},
		},
		{
			name:     "in progress",
			data:     map[string]string{PlanStateKey: string(PlanStateInProgress)},
			expected: PlanStatus{InProgress: true},
		},
		{
			name: "in progress retrying an earlier failure",
			data: map[string]string{
				PlanStateKey: string(PlanStateInProgress), failedChecksumKey: checksum, failureCountKey: "1", failureThresholdKey: "3",
			},
			expected: PlanStatus{InProgress: true, Failing: true},
		},
		{
			name:     "succeeded, waiting for probes",
			data:     map[string]string{PlanStateKey: string(PlanStateSucceeded)},
			expected: PlanStatus{Applied: true},
		},
		{
			name:        "succeeded and probes passed",
			data:        map[string]string{PlanStateKey: string(PlanStateSucceeded), probeStatusesKey: string(healthyProbes)},
			annotations: map[string]string{PlanProbesPassedAnnotation: "yes"},
			expected:    PlanStatus{Applied: true, ProbesPassed: true},
		},
		{
			name: "failed at the threshold",
			data: map[string]string{
				PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1",
				maxFailuresKey: "1", failureThresholdKey: "1",
			},
			expected: PlanStatus{Failed: true},
		},
		{
			name:     "failed without a threshold set",
			data:     map[string]string{PlanStateKey: string(PlanStateFailed)},
			expected: PlanStatus{Failed: true},
		},
		{
			name: "failed below the threshold is retried",
			data: map[string]string{
				PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "2",
				maxFailuresKey: "5", failureThresholdKey: "5",
			},
			expected: PlanStatus{Pending: true, Failing: true},
			retried:  true,
		},
		{
			name: "failed below the threshold waits out the cooldown",
			data: map[string]string{
				PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "2",
				maxFailuresKey: "5", failureThresholdKey: "5", lastApplyTimeKey: fixed.Add(-10 * time.Second).Format(time.UnixDate),
			},
			expected: PlanStatus{Failing: true},
		},
		{
			name: "failed below the threshold is retried once the cooldown passes",
			data: map[string]string{
				PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "2",
				maxFailuresKey: "5", failureThresholdKey: "5", lastApplyTimeKey: fixed.Add(-time.Minute).Format(time.UnixDate),
			},
			expected: PlanStatus{Pending: true, Failing: true},
			retried:  true,
		},
		{
			name: "failed out of attempts with an unlimited threshold is not retried",
			data: map[string]string{
				PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "3",
				maxFailuresKey: "3", failureThresholdKey: "-1",
			},
			expected: PlanStatus{Failing: true},
		},
		{
			name: "failed and canceled is not retried",
			data: map[string]string{
				PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1",
				maxFailuresKey: "5", failureThresholdKey: "5",
			},
			annotations: map[string]string{PlanCanceledAnnotation: "true"},
			expected:    PlanStatus{Canceled: true},
		},
		{
			name:     "canceled",
			data:     map[string]string{PlanStateKey: string(PlanStateCanceled)},
			expected: PlanStatus{Canceled: true},
		},
		{
			name:     "paused",
			data:     map[string]string{PlanStateKey: string(PlanStatePaused)},
			expected: PlanStatus{Paused: true},
		},
		// Agents which predate plan-state leave it as pending, or absent for a plan assigned before it existed.
		{
			name:     "checksum flow applied",
			data:     map[string]string{PlanStateKey: string(PlanStatePending), appliedPlanKey: string(raw)},
			expected: PlanStatus{Applied: true},
		},
		{
			name:     "checksum flow without plan-state in progress",
			data:     map[string]string{},
			expected: PlanStatus{InProgress: true},
		},
		{
			name: "checksum flow failing",
			data: map[string]string{
				PlanStateKey: string(PlanStatePending), failedChecksumKey: checksum, failureCountKey: "1", failureThresholdKey: "3",
			},
			expected: PlanStatus{InProgress: true, Failing: true},
		},
		{
			name: "checksum flow failed",
			data: map[string]string{
				PlanStateKey: string(PlanStatePending), failedChecksumKey: checksum, failureCountKey: "3", failureThresholdKey: "3",
			},
			expected: PlanStatus{Failed: true},
		},
		{
			name: "checksum flow ignores failures of another plan",
			data: map[string]string{
				PlanStateKey: string(PlanStatePending), failedChecksumKey: "other", failureCountKey: "3", failureThresholdKey: "3",
			},
			expected: PlanStatus{Pending: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeSecretUpdater{}
			status, err := NewStore(client).AssignPlan(withPlan(tt.data, tt.annotations), assigned, 5, 5)
			require.NoError(t, err)

			tt.expected.Secret = status.Secret
			assert.Equal(t, &tt.expected, status)

			if !tt.retried {
				assert.Empty(t, client.updates)
				return
			}
			require.Len(t, client.updates, 1)
			assert.Equal(t, string(PlanStatePending), string(client.updates[0].Data[PlanStateKey]))
			assert.Equal(t, raw, client.updates[0].Data[PlanDataKey], "a retry must not change the plan")
		})
	}
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

// secretWithData returns a secret carrying the given data, already holding raw as its plan.
func secretWithData(raw []byte, data map[string]string) *corev1.Secret {
	s := mockSecret("node-alpha")
	s.Data = map[string][]byte{PlanDataKey: raw}
	for k, v := range data {
		s.Data[k] = []byte(v)
	}
	s.Annotations = map[string]string{}
	return s
}

func TestPlanStatusMethods(t *testing.T) {
	tests := []struct {
		name    string
		status  PlanStatus
		success bool
		failure bool
		waiting bool
		message string
	}{
		{name: "pending", status: PlanStatus{Pending: true}, waiting: true, message: "waiting for plan to be picked up"},
		{name: "in progress", status: PlanStatus{InProgress: true}, waiting: true, message: "waiting for plan to be applied"},
		{name: "paused", status: PlanStatus{Paused: true}, waiting: true, message: "plan paused"},
		{name: "applied awaiting probes", status: PlanStatus{Applied: true}, waiting: true, message: "waiting for probes"},
		{name: "applied and probes passed", status: PlanStatus{Applied: true, ProbesPassed: true}, success: true, message: "plan successfully applied"},
		{name: "failing", status: PlanStatus{Failing: true}, waiting: true, message: "waiting for plan to succeed or reach failure limit"},
		{name: "failing while retry is pending", status: PlanStatus{Failing: true, Pending: true}, waiting: true, message: "waiting for plan to be picked up"},
		{name: "failed", status: PlanStatus{Failed: true}, failure: true, message: "plan failed to be applied"},
		{name: "canceled", status: PlanStatus{Canceled: true}, failure: true, message: "plan canceled"},
		{name: "zero value", status: PlanStatus{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.success, tt.status.Success(), "Success")
			assert.Equal(t, tt.failure, tt.status.Failure(), "Failure")
			assert.Equal(t, tt.waiting, tt.status.Waiting(), "Waiting")
			assert.Equal(t, tt.message, tt.status.String(), "String")
			// A plan is never both done and still to be waited on, so a caller checking Failure and
			// then Waiting can never wait on a plan that will not move again.
			assert.False(t, tt.status.Waiting() && (tt.status.Success() || tt.status.Failure()), "terminal and waiting at once")
		})
	}
}

func TestMessagePausedAndCanceled(t *testing.T) {
	assert.Equal(t, "waiting for plan applied for node-a, plan paused for node-b & 1 other node, waiting for probes for node-d",
		Message([]PlanStatus{
			{Secret: mockSecret("node-d"), Applied: true},
			{Secret: mockSecret("node-c"), Paused: true},
			{Secret: mockSecret("node-b"), Paused: true},
			{Secret: mockSecret("node-a"), InProgress: true},
			{Secret: mockSecret("node-e"), Canceled: true},
		}))
	assert.Equal(t, "", Message([]PlanStatus{{Secret: mockSecret("node-a"), Canceled: true}}), "a canceled plan has nothing left to wait on")
}

func TestMessageUsesMachineName(t *testing.T) {
	s := mockSecret("secret-name")
	s.Labels = map[string]string{planv1alpha1.MachineLifecycleNameLabel: "machine-name"}
	assert.Equal(t, "waiting for plan applied for machine-name", Message([]PlanStatus{{Secret: s, InProgress: true}}))
}

func TestStoreAssignPlanNewContent(t *testing.T) {
	assigned, raw := testPlan(t, "true")

	t.Run("initializes a secret with no data or annotations", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(mockSecret("node-alpha"), assigned, 0, 0)
		require.NoError(t, err)
		assert.True(t, status.Pending)
		require.Len(t, client.updates, 1)
		written := client.updates[0]
		assert.Equal(t, raw, written.Data[PlanDataKey])
		assert.Equal(t, string(PlanStatePending), string(written.Data[PlanStateKey]))
		assert.NotEmpty(t, written.Annotations[PlanLastUpdatedAnnotation])
	})

	t.Run("unset failure limits remove the previous plan's", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := secretWithData([]byte("previous"), map[string]string{maxFailuresKey: "5", failureThresholdKey: "5"})
		_, err := NewStore(client).AssignPlan(secret, assigned, 0, 0)
		require.NoError(t, err)
		require.Len(t, client.updates, 1)
		assert.NotContains(t, client.updates[0].Data, maxFailuresKey)
		assert.NotContains(t, client.updates[0].Data, failureThresholdKey)
	})

	t.Run("unlimited failure limits are written", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		_, err := NewStore(client).AssignPlan(mockSecret("node-alpha"), assigned, -1, -1)
		require.NoError(t, err)
		require.Len(t, client.updates, 1)
		assert.Equal(t, "-1", string(client.updates[0].Data[maxFailuresKey]))
		assert.Equal(t, "-1", string(client.updates[0].Data[failureThresholdKey]))
	})

	// Stale failure records of an earlier plan with the same hash must not fail a fresh assignment
	// before the agent has had the chance to run it.
	t.Run("reports pending regardless of what earlier plans recorded", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := secretWithData([]byte("previous"), map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: PlanHash(raw), failureCountKey: "9",
		})
		status, err := NewStore(client).AssignPlan(secret, assigned, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, &PlanStatus{Secret: status.Secret, Pending: true}, status)
	})

	t.Run("does not mutate the secret it was given", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := secretWithData([]byte("previous"), map[string]string{PlanStateKey: string(PlanStateSucceeded)})
		secret.Annotations[PlanCanceledAnnotation] = "true"
		_, err := NewStore(client).AssignPlan(secret, assigned, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, "previous", string(secret.Data[PlanDataKey]))
		assert.Equal(t, string(PlanStateSucceeded), string(secret.Data[PlanStateKey]))
		assert.Equal(t, "true", secret.Annotations[PlanCanceledAnnotation])
	})

	t.Run("returns the update error", func(t *testing.T) {
		client := &fakeSecretUpdater{err: errors.New("boom")}
		status, err := NewStore(client).AssignPlan(mockSecret("node-alpha"), assigned, 1, 1)
		assert.Error(t, err)
		assert.Nil(t, status)
	})
}

// A follow-up which assigns content byte-identical to the plan already on the secret is not a new
// plan: it is reported on the strength of whatever the previous assignment left behind, and nothing
// is written. This is why every operation scopes its plans with ops.WithOperationEnv — and why a
// follow-up whose plan differs in any way is run afresh.
func TestStoreAssignPlanFollowUp(t *testing.T) {
	assigned, raw := testPlan(t, "true")
	followUp, followUpRaw := testPlan(t, "false")

	for _, previous := range []PlanState{PlanStateSucceeded, PlanStateFailed, PlanStateCanceled, PlanStateInProgress, PlanStatePaused} {
		t.Run("identical content after "+string(previous)+" is not reassigned", func(t *testing.T) {
			client := &fakeSecretUpdater{}
			secret := secretWithData(raw, map[string]string{PlanStateKey: string(previous), failureThresholdKey: "1"})
			secret.Annotations[PlanCanceledAnnotation] = "true"

			status, err := NewStore(client).AssignPlan(secret, assigned, 1, 1)
			require.NoError(t, err)
			assert.Empty(t, client.updates)
			assert.False(t, status.Pending, "an identical plan is never handed to the agent again")
		})

		t.Run("distinct content after "+string(previous)+" is reassigned", func(t *testing.T) {
			client := &fakeSecretUpdater{}
			secret := secretWithData(raw, map[string]string{
				PlanStateKey: string(previous), appliedPlanKey: string(raw), probeStatusesKey: `{"p":{"healthy":true}}`,
				failedChecksumKey: PlanHash(raw), failureCountKey: "1",
			})
			secret.Annotations[PlanCanceledAnnotation] = "true"
			secret.Annotations[PlanProbesPassedAnnotation] = "yes"

			status, err := NewStore(client).AssignPlan(secret, followUp, 1, 1)
			require.NoError(t, err)
			assert.Equal(t, &PlanStatus{Secret: status.Secret, Pending: true}, status)
			require.Len(t, client.updates, 1)
			written := client.updates[0]
			assert.Equal(t, followUpRaw, written.Data[PlanDataKey])
			assert.Equal(t, string(PlanStatePending), string(written.Data[PlanStateKey]))
			assert.NotContains(t, written.Annotations, PlanCanceledAnnotation, "the follow-up must not inherit the cancellation")
			assert.Equal(t, "", written.Annotations[PlanProbesPassedAnnotation])
			assert.NotContains(t, written.Data, probeStatusesKey)

			// Once the agent picks it up, the follow-up is judged on its own outcome: the previous
			// plan's applied plan and failure records do not describe it.
			written.Data[PlanStateKey] = []byte(PlanStateInProgress)
			status, err = NewStore(&fakeSecretUpdater{}).AssignPlan(written, followUp, 1, 1)
			require.NoError(t, err)
			assert.Equal(t, &PlanStatus{Secret: status.Secret, InProgress: true}, status)
		})
	}

	// Spelled out, as these are the two outcomes a follow-up would be wrongly handed.
	t.Run("identical content after a cancellation reports canceled", func(t *testing.T) {
		status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(
			secretWithData(raw, map[string]string{PlanStateKey: string(PlanStateCanceled)}), assigned, 1, 1)
		require.NoError(t, err)
		assert.True(t, status.Canceled)
		assert.True(t, status.Failure())
	})
	t.Run("identical content after a success reports applied", func(t *testing.T) {
		status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(
			secretWithData(raw, map[string]string{PlanStateKey: string(PlanStateSucceeded)}), assigned, 1, 1)
		require.NoError(t, err)
		assert.True(t, status.Applied)
	})
}

func TestStoreAssignPlanRetry(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	now = func() time.Time { return fixed }
	t.Cleanup(func() { now = time.Now })

	assigned, raw := testPlan(t, "true")
	checksum := PlanHash(raw)

	t.Run("resets the previous attempt's probes", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		secret := secretWithData(raw, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1",
			maxFailuresKey: "3", failureThresholdKey: "3", probeStatusesKey: `{"p":{"healthy":true}}`,
		})
		secret.Annotations[PlanProbesPassedAnnotation] = "yes"

		status, err := NewStore(client).AssignPlan(secret, assigned, 3, 3)
		require.NoError(t, err)
		assert.True(t, status.Pending)
		assert.True(t, status.Failing)
		require.Len(t, client.updates, 1)
		written := client.updates[0]
		assert.NotContains(t, written.Data, probeStatusesKey)
		assert.Equal(t, "", written.Annotations[PlanProbesPassedAnnotation])
		assert.Equal(t, fixed.Format(time.RFC3339), written.Annotations[PlanLastUpdatedAnnotation])
		assert.Equal(t, "1", string(written.Data[failureCountKey]), "the failure count is the agent's to keep")
		assert.Equal(t, written, status.Secret, "the status must carry the secret as written")
	})

	retried := func(t *testing.T, data map[string]string) bool {
		t.Helper()
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(secretWithData(raw, data), assigned, 3, 3)
		require.NoError(t, err)
		assert.True(t, status.Failing, "a plan with attempts left is failing, not failed")
		assert.False(t, status.Failed)
		return len(client.updates) == 1 && status.Pending
	}

	t.Run("retries with unlimited attempts", func(t *testing.T) {
		assert.True(t, retried(t, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "40",
			maxFailuresKey: "-1", failureThresholdKey: "-1",
		}))
	})
	t.Run("retries when max-failures is unset", func(t *testing.T) {
		assert.True(t, retried(t, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "2", failureThresholdKey: "3",
		}))
	})
	t.Run("retries when last-apply-time is unparsable", func(t *testing.T) {
		assert.True(t, retried(t, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1",
			maxFailuresKey: "3", failureThresholdKey: "3", lastApplyTimeKey: "yesterday",
		}))
	})
	t.Run("retries exactly once the cooldown has passed", func(t *testing.T) {
		assert.True(t, retried(t, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1",
			maxFailuresKey: "3", failureThresholdKey: "3", lastApplyTimeKey: fixed.Add(-failedPlanRetryCooldown).Format(time.UnixDate),
		}))
	})
	// plan-state failed is authoritative even if the failure records belong to another plan.
	t.Run("counts a failure recorded against another plan as one", func(t *testing.T) {
		assert.True(t, retried(t, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: "other", failureCountKey: "9",
			maxFailuresKey: "3", failureThresholdKey: "3",
		}))
	})

	t.Run("does not retry a failure recorded against another plan with a threshold of one", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(secretWithData(raw, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: "other", failureCountKey: "9", failureThresholdKey: "1",
		}), assigned, 1, 1)
		require.NoError(t, err)
		assert.True(t, status.Failed)
		assert.Empty(t, client.updates)
	})

	t.Run("does not retry a plan that reached its threshold", func(t *testing.T) {
		client := &fakeSecretUpdater{}
		status, err := NewStore(client).AssignPlan(secretWithData(raw, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "3",
			maxFailuresKey: "5", failureThresholdKey: "3",
		}), assigned, 5, 3)
		require.NoError(t, err)
		assert.Equal(t, &PlanStatus{Secret: status.Secret, Failed: true}, status)
		assert.Empty(t, client.updates)
	})

	t.Run("returns the update error", func(t *testing.T) {
		client := &fakeSecretUpdater{err: errors.New("boom")}
		status, err := NewStore(client).AssignPlan(secretWithData(raw, map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1",
			maxFailuresKey: "3", failureThresholdKey: "3",
		}), assigned, 3, 3)
		assert.Error(t, err)
		assert.Nil(t, status)
	})

	// The agent records a retry it has picked up as in-progress; the retry is then reported as both,
	// and once it succeeds the plan's earlier failures no longer matter.
	t.Run("a retry runs to success", func(t *testing.T) {
		secret := secretWithData(raw, map[string]string{
			PlanStateKey: string(PlanStateInProgress), failedChecksumKey: checksum, failureCountKey: "1",
			maxFailuresKey: "3", failureThresholdKey: "3",
		})
		status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(secret, assigned, 3, 3)
		require.NoError(t, err)
		assert.Equal(t, &PlanStatus{Secret: status.Secret, InProgress: true, Failing: true}, status)

		secret.Data[PlanStateKey] = []byte(PlanStateSucceeded)
		secret.Data[failureCountKey] = []byte("0")
		secret.Data[failedChecksumKey] = []byte{}
		secret.Data[probeStatusesKey] = []byte(`{"p":{"healthy":true}}`)
		secret.Annotations[PlanProbesPassedAnnotation] = "yes"
		status, err = NewStore(&fakeSecretUpdater{}).AssignPlan(secret, assigned, 3, 3)
		require.NoError(t, err)
		assert.True(t, status.Success())
	})
}

func TestStoreAssignPlanProbes(t *testing.T) {
	assigned, raw := testPlan(t, "true")

	tests := []struct {
		name         string
		probes       string
		probesPassed string
		expected     bool
	}{
		{name: "healthy probes that have passed", probes: `{"a":{"healthy":true},"b":{"healthy":true}}`, probesPassed: "yes", expected: true},
		{name: "one unhealthy probe", probes: `{"a":{"healthy":true},"b":{}}`, probesPassed: "yes"},
		{name: "healthy probes not yet recorded as passed", probes: `{"a":{"healthy":true}}`},
		{name: "passed but no probe statuses", probesPassed: "yes"},
	}

	for _, tt := range tests {
		for _, state := range []PlanState{PlanStateSucceeded, ""} {
			t.Run(tt.name+" with plan-state "+string(state), func(t *testing.T) {
				data := map[string]string{appliedPlanKey: string(raw)}
				if state != "" {
					data[PlanStateKey] = string(state)
				}
				if tt.probes != "" {
					data[probeStatusesKey] = tt.probes
				}
				secret := secretWithData(raw, data)
				if tt.probesPassed != "" {
					secret.Annotations[PlanProbesPassedAnnotation] = tt.probesPassed
				}

				status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(secret, assigned, 1, 1)
				require.NoError(t, err)
				assert.True(t, status.Applied)
				assert.Equal(t, tt.expected, status.ProbesPassed)
			})
		}
	}
}

func TestStoreAssignPlanMalformedData(t *testing.T) {
	assigned, raw := testPlan(t, "true")
	checksum := PlanHash(raw)

	tests := []struct {
		name string
		data map[string]string
	}{
		{name: "probe statuses", data: map[string]string{PlanStateKey: string(PlanStateSucceeded), probeStatusesKey: "{"}},
		{name: "probe statuses in the checksum flow", data: map[string]string{probeStatusesKey: "{"}},
		{name: "failure count while in progress", data: map[string]string{
			PlanStateKey: string(PlanStateInProgress), failedChecksumKey: checksum, failureCountKey: "x",
		}},
		{name: "failure count of a failed plan", data: map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "x",
		}},
		{name: "failure threshold", data: map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1", failureThresholdKey: "x",
		}},
		{name: "max failures", data: map[string]string{
			PlanStateKey: string(PlanStateFailed), failedChecksumKey: checksum, failureCountKey: "1", failureThresholdKey: "3", maxFailuresKey: "x",
		}},
		{name: "failure count in the checksum flow", data: map[string]string{failedChecksumKey: checksum, failureCountKey: "x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := secretWithData(raw, tt.data)
			secret.Annotations[PlanProbesPassedAnnotation] = "yes"
			client := &fakeSecretUpdater{}
			status, err := NewStore(client).AssignPlan(secret, assigned, 3, 3)
			assert.Error(t, err)
			assert.Nil(t, status)
			assert.Empty(t, client.updates)
		})
	}

	// Malformed failure records of another plan are never read.
	t.Run("failure count of another plan", func(t *testing.T) {
		status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(secretWithData(raw, map[string]string{
			PlanStateKey: string(PlanStateInProgress), failedChecksumKey: "other", failureCountKey: "x",
		}), assigned, 3, 3)
		require.NoError(t, err)
		assert.True(t, status.InProgress)
	})
}

// An agent may write a state this build does not know about yet. It is treated like a secret
// without plan-state rather than trusted to mean anything in particular.
func TestStoreAssignPlanUnknownState(t *testing.T) {
	assigned, raw := testPlan(t, "true")

	status, err := NewStore(&fakeSecretUpdater{}).AssignPlan(
		secretWithData(raw, map[string]string{PlanStateKey: "from-the-future"}), assigned, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, &PlanStatus{Secret: status.Secret, InProgress: true}, status)

	status, err = NewStore(&fakeSecretUpdater{}).AssignPlan(
		secretWithData(raw, map[string]string{PlanStateKey: "from-the-future", appliedPlanKey: string(raw)}), assigned, 1, 1)
	require.NoError(t, err)
	assert.True(t, status.Applied)
}
