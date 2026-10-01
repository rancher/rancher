package plan

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

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
