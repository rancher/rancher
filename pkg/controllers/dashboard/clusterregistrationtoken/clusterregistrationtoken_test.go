package clusterregistrationtoken

import (
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// test helpers
const (
	staleCurrentExpiresAt            = "stale-current-expires-at"
	staleCurrentGracePeriodExpiresAt = "stale-current-grace-period-expires-at"
)

func ptrInt64(v int64) *int64 { return &v }

func newTestCRT(creationTime time.Time, ttl, gracePeriod *int64, currentExpiresAt, currentGracePeriodExpiresAt string) *v3.ClusterRegistrationToken {
	return &v3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         "c-test",
			Name:              "test-crt",
			CreationTimestamp: metav1.NewTime(creationTime),
		},
		Spec: v3.ClusterRegistrationTokenSpec{
			TTL:         ttl,
			GracePeriod: gracePeriod,
		},
		Status: v3.ClusterRegistrationTokenStatus{
			ExpiresAt:            currentExpiresAt,
			GracePeriodExpiresAt: currentGracePeriodExpiresAt,
		},
	}
}

func setTTLEnabled(t *testing.T, enabled bool) {
	t.Helper()
	features.CRTTokenTTLRotation.Set(enabled)
	t.Cleanup(features.CRTTokenTTLRotation.Unset)
}

// newHandlerForSecret builds a *handler backed by mock SecretCache/SecretClient
// that serve the given Secret and capture the single expected Update call.
// The returned func retrieves whatever was passed to Update (nil if never called).
func newHandlerForSecret(t *testing.T, secret *corev1.Secret) (*handler, func() *corev1.Secret) {
	t.Helper()
	ctrl := gomock.NewController(t)
	secretCache := fake.NewMockCacheInterface[*corev1.Secret](ctrl)
	secretCache.EXPECT().Get(secret.Namespace, secret.Name).Return(secret, nil)
	secretClient := fake.NewMockControllerInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	var updated *corev1.Secret
	secretClient.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
		updated = s
		return s, nil
	})
	h := &handler{secretCache: secretCache, secrets: secretClient}
	return h, func() *corev1.Secret { return updated }
}

// Grace period cleanup must fire even when TTL rotation is disabled - a
// promised previousToken expiry must still be honored.
func TestComputeTTLDecision_GracePeriodOrderingBeforeTTLGate(t *testing.T) {
	setTTLEnabled(t, false)
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	past := now.Add(-1 * time.Hour).UTC().Format(time.RFC3339)

	data := map[string][]byte{
		tokenDataKey:                []byte("tok"),
		previousTokenDataKey:        []byte("old-tok"),
		gracePeriodExpiresAtDataKey: []byte(past),
	}

	obj := newTestCRT(now, ptrInt64(60), ptrInt64(10), staleCurrentExpiresAt, staleCurrentGracePeriodExpiresAt)
	d, err := computeTTLDecision(now, obj, data)
	require.NoError(t, err)
	require.Contains(t, d.deleteKeys, previousTokenDataKey)
	require.Contains(t, d.deleteKeys, gracePeriodExpiresAtDataKey)
}

// When there's nothing to do (TTL disabled, or ttl<=0 "never expire"), the
// current status must be carried forward untouched - never silently cleared.
func TestComputeTTLDecision_NeverSilentlyClearsCurrentStatus(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(1 * time.Hour).UTC().Format(time.RFC3339)

	cases := []struct {
		name       string
		data       map[string][]byte
		ttlEnabled bool
		ttl        int64
	}{
		{name: "ttl disabled, no schedule", data: map[string][]byte{tokenDataKey: []byte("tok")}, ttlEnabled: false, ttl: 60},
		{name: "ttl disabled, expiresAt present", data: map[string][]byte{tokenDataKey: []byte("tok"), expiresAtDataKey: []byte(future)}, ttlEnabled: false, ttl: 60},
		{name: "ttl<=0 means never expire", data: map[string][]byte{tokenDataKey: []byte("tok")}, ttlEnabled: true, ttl: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setTTLEnabled(t, tc.ttlEnabled)
			obj := newTestCRT(now.Add(-time.Hour), ptrInt64(tc.ttl), ptrInt64(10), staleCurrentExpiresAt, staleCurrentGracePeriodExpiresAt)
			d, err := computeTTLDecision(now, obj, tc.data)
			require.NoError(t, err)
			require.False(t, d.writesSecret(), "no Secret write should happen when nothing was decided")
			require.Equal(t, staleCurrentExpiresAt, d.expiresAt)
			require.Equal(t, staleCurrentGracePeriodExpiresAt, d.gracePeriodExpiresAt)
		})
	}
}

// Feeds computeTTLDecision's own output back into itself across a full
// lifecycle (stamp -> wait -> rotate -> grace wait -> cleanup -> next
// window), asserting requeue timing is correct at each step and nothing
// ever rotates or writes twice for the same instant.
func TestTTLDecisionConvergesAndIsIdempotent(t *testing.T) {
	setTTLEnabled(t, true)
	const ttl = int64(60)
	const gracePeriod = int64(10)
	creationTime := time.Now().Add(-48 * time.Hour)
	now := time.Now()

	data := map[string][]byte{tokenDataKey: []byte("initial-token")}
	obj := newTestCRT(creationTime, ptrInt64(ttl), ptrInt64(gracePeriod), "", "")

	apply := func(d ttlDecision) {
		for k, v := range d.setData {
			data[k] = v
		}
		for _, k := range d.deleteKeys {
			delete(data, k)
		}
		obj.Status.ExpiresAt = d.expiresAt
		obj.Status.GracePeriodExpiresAt = d.gracePeriodExpiresAt
	}

	// Initial stamp: no schedule yet, one gets assigned, no requeue (the
	// resulting Secret write triggers the next reconcile).
	d1, err := computeTTLDecision(now, obj, data)
	require.NoError(t, err)
	require.NotEmpty(t, d1.expiresAt)
	require.Zero(t, d1.requeueAfter)
	apply(d1)

	// Immediate re-reconcile: must be a no-op write, and must schedule the
	// wakeup at expiry.
	d2, err := computeTTLDecision(now, obj, data)
	require.NoError(t, err)
	require.Empty(t, d2.setData, "immediate re-reconcile at the same instant must be a no-op write")
	require.Equal(t, obj.Status.ExpiresAt, d2.expiresAt)
	require.Greater(t, d2.requeueAfter, time.Duration(0))
	apply(d2)

	// Expiry reached: rotate. New token issued, old token preserved as
	// previousToken, no requeue (rotation itself doesn't requeue).
	expiredNow := now.Add(time.Duration(ttl)*time.Minute + 20*time.Minute)
	oldToken := string(data[tokenDataKey])
	d3, err := computeTTLDecision(expiredNow, obj, data)
	require.NoError(t, err)
	require.NotEmpty(t, d3.setData[tokenDataKey])
	require.NotEqual(t, oldToken, string(d3.setData[tokenDataKey]))
	require.Equal(t, []byte(oldToken), d3.setData[previousTokenDataKey])
	require.Zero(t, d3.requeueAfter)
	apply(d3)

	// Right after rotation: no further write, grace period wakeup scheduled.
	d4, err := computeTTLDecision(expiredNow, obj, data)
	require.NoError(t, err)
	require.Empty(t, d4.setData)
	require.Empty(t, d4.deleteKeys)
	require.Greater(t, d4.requeueAfter, time.Duration(0))
	apply(d4)

	// Repeated reconciles while grace period is still active: must never
	// rotate again.
	rotatedToken := string(data[tokenDataKey])
	for i := 0; i < 3; i++ {
		dRepeat, err := computeTTLDecision(expiredNow, obj, data)
		require.NoError(t, err)
		require.Empty(t, dRepeat.setData, "must not rotate again while grace period is still active")
		require.Equal(t, rotatedToken, string(data[tokenDataKey]))
	}

	// Grace period expires: previousToken and gracePeriodExpiresAt are
	// cleaned up, ExpiresAt carried forward unchanged, no requeue.
	graceExpiredNow := expiredNow.Add(time.Duration(gracePeriod)*time.Minute + time.Minute)
	d5, err := computeTTLDecision(graceExpiredNow, obj, data)
	require.NoError(t, err)
	require.Contains(t, d5.deleteKeys, previousTokenDataKey)
	require.Contains(t, d5.deleteKeys, gracePeriodExpiresAtDataKey)
	require.Zero(t, d5.requeueAfter)
	require.Equal(t, obj.Status.ExpiresAt, d5.expiresAt)
	apply(d5)
	require.NotContains(t, data, previousTokenDataKey)
	require.NotContains(t, data, gracePeriodExpiresAtDataKey)
	require.Empty(t, obj.Status.GracePeriodExpiresAt)

	// After cleanup: no-op write, next rotation window's wakeup is scheduled.
	d6, err := computeTTLDecision(graceExpiredNow, obj, data)
	require.NoError(t, err)
	require.Empty(t, d6.setData)
	require.Greater(t, d6.requeueAfter, time.Duration(0))
}

// A stale CRT.Status.ExpiresAt (e.g. left over from before the feature was
// disabled) must never be reused; handleTTL should stamp a fresh schedule there.
func TestHandleTTL_ReadsFromSecretNotStaleStatus(t *testing.T) {
	features.CRTTokenTTLRotation.Set(true)
	defer features.CRTTokenTTLRotation.Unset()

	secretName := SecretName("system")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: secretName},
		Data: map[string][]byte{
			tokenDataKey: []byte("tok"),
			// No expiresAtDataKey: Secret has no schedule yet.
		},
	}

	h, updated := newHandlerForSecret(t, secret)

	crt := &v3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "system", CreationTimestamp: metav1.NewTime(time.Now().Add(-48 * time.Hour))},
		Status: v3.ClusterRegistrationTokenStatus{
			TokenSecretName: secretName,
			// Stale: leftover from before the feature was disabled.
			ExpiresAt: "2020-01-01T00:00:00Z",
		},
	}

	_, err := h.handleTTL(crt)
	require.NoError(t, err)

	require.NotNil(t, updated(), "a fresh expiresAt must be persisted to the Secret")
	require.NotEqual(t, "2020-01-01T00:00:00Z", string(updated().Data[expiresAtDataKey]), "stale Status.ExpiresAt must not be reused as the new schedule")
	require.NotEmpty(t, updated().Data[expiresAtDataKey])
}

// When the feature is disabled, handleTTL must not assign a new expiresAt,
// but must still clean up an already-expired grace period from the Secret.
func TestHandleTTL_DisabledFeatureHonorsExistingGracePeriod(t *testing.T) {
	features.CRTTokenTTLRotation.Set(false)
	defer features.CRTTokenTTLRotation.Unset()

	secretName := SecretName("system")
	graceAt := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339) // already expired
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: secretName},
		Data: map[string][]byte{
			tokenDataKey:                []byte("tok"),
			previousTokenDataKey:        []byte("old-tok"),
			gracePeriodExpiresAtDataKey: []byte(graceAt),
		},
	}

	h, updated := newHandlerForSecret(t, secret)

	crt := &v3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{Namespace: "c-abc", Name: "system"},
		Status:     v3.ClusterRegistrationTokenStatus{TokenSecretName: secretName},
	}

	_, err := h.handleTTL(crt)
	require.NoError(t, err)

	require.NotNil(t, updated(), "expired grace period must be cleaned up from the Secret even when the feature is disabled")
	require.Empty(t, updated().Data[gracePeriodExpiresAtDataKey])
	require.Empty(t, updated().Data[expiresAtDataKey], "feature disabled: no new expiry should be assigned")
}
