package tokens

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// MaxTTL is the hard limit, in milliseconds, on the time-to-live of a token: 10 years
// (3650 days). It applies on top of the max TTL settings, including when those allow
// no limit. Negative TTLs (never expires) are not affected by it, where the settings
// allow them.
const MaxTTL = int64(3650 * 24 * time.Hour / time.Millisecond)

// ErrTTLExceedsMax is returned when a token time-to-live is above MaxTTL.
var ErrTTLExceedsMax = errors.New("ttl exceeds the maximum of 10 years")

// ValidateTTL checks a resolved token time-to-live (milliseconds, see IngestTTL)
// against MaxTTL. It returns an error wrapping ErrTTLExceedsMax if it is above.
func ValidateTTL(ttl int64) error {
	if ttl > MaxTTL {
		return fmt.Errorf("%w: %d > %d milliseconds", ErrTTLExceedsMax, ttl, MaxTTL)
	}
	return nil
}

// TTLToDuration converts a time-to-live in milliseconds into a time.Duration. Values
// outside of the range of time.Duration (about 292 years) saturate to its limits
// instead of overflowing, which would turn a very long time-to-live into an
// arbitrary, possibly negative, duration.
func TTLToDuration(ttl int64) time.Duration {
	const limit = math.MaxInt64 / int64(time.Millisecond)
	switch {
	case ttl > limit:
		return math.MaxInt64
	case ttl < -limit:
		return math.MinInt64
	}
	return time.Duration(ttl) * time.Millisecond
}
