package plan

import (
	"errors"
	"testing"

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
