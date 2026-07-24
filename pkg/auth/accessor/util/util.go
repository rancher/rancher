package util

import (
	"time"

	"github.com/rancher/rancher/pkg/auth/accessor"
	"github.com/rancher/rancher/pkg/auth/settings"
)

// IsIdleExpired returns true if last recorded user activity is past the idle timeout.
func IsIdleExpired(t accessor.TokenAccessor, now time.Time) bool {
	activityLastSeen := t.GetLastActivitySeen()

	if activityLastSeen.IsZero() {
		return false
	}

	idleTimeout := settings.AuthUserSessionIdleTTLMinutes.GetInt()
	if idleTimeout == 0 {
		return false
	}

	return now.After(activityLastSeen.Add(time.Duration(idleTimeout) * time.Minute))
}
