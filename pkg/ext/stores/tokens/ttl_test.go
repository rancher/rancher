package tokens

import (
	"math"
	"testing"
	"time"

	ext "github.com/rancher/rancher/pkg/apis/ext.cattle.io/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMaxTTL(t *testing.T) {
	assert.Equal(t, int64(315_360_000_000), MaxTTL)
}

func TestValidateTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     int64
		wantErr bool
	}{
		{name: "never expires", ttl: -1},
		{name: "default", ttl: 0},
		{name: "one day", ttl: int64(24 * time.Hour / time.Millisecond)},
		{name: "at the limit", ttl: MaxTTL},
		{name: "above the limit", ttl: MaxTTL + 1, wantErr: true},
		{name: "above the range of time.Duration", ttl: 10_000_000_000_000, wantErr: true},
		{name: "max int64", ttl: math.MaxInt64, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateTTL(test.ttl)
			if test.wantErr {
				assert.ErrorIs(t, err, ErrTTLExceedsMax)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestTTLToDuration(t *testing.T) {
	const limit = math.MaxInt64 / int64(time.Millisecond)

	tests := []struct {
		name string
		ttl  int64
		want time.Duration
	}{
		{name: "zero", ttl: 0, want: 0},
		{name: "one second", ttl: 1000, want: time.Second},
		{name: "negative", ttl: -1000, want: -time.Second},
		{name: "upper limit", ttl: limit, want: time.Duration(limit) * time.Millisecond},
		{name: "lower limit", ttl: -limit, want: time.Duration(-limit) * time.Millisecond},
		{name: "above the upper limit saturates", ttl: limit + 1, want: math.MaxInt64},
		{name: "wraps to negative without saturation", ttl: 10_000_000_000_000, want: math.MaxInt64},
		{name: "wraps to small positive without saturation", ttl: 18_446_744_073_710, want: math.MaxInt64},
		{name: "below the lower limit saturates", ttl: -limit - 1, want: math.MinInt64},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, TTLToDuration(test.ttl))
		})
	}
}

// TestSetExpiredLongTTL checks that a TTL above the range of time.Duration, as tokens stored
// before MaxTTL existed may have, doesn't wrap around into an expiry in the past.
func TestSetExpiredLongTTL(t *testing.T) {
	created := time.Date(2026, time.November, 9, 0, 0, 0, 0, time.UTC)
	token := &ext.Token{
		ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(created)},
		Spec:       ext.TokenSpec{TTL: 10_000_000_000_000}, // about 317 years
	}

	require.NoError(t, setExpired(token))

	assert.False(t, token.Status.Expired)
	expiresAt, err := time.Parse(time.RFC3339, token.Status.ExpiresAt)
	require.NoError(t, err)
	assert.True(t, expiresAt.After(created.Add(290*365*24*time.Hour)), "expiresAt %s", token.Status.ExpiresAt)
}
