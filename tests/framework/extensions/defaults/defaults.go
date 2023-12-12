package defaults

import "time"

var (
	WatchTimeoutSeconds  = int64(60 * 30) // 30 minutes.
	FiveMinuteTimeout    = 5 * time.Minute
	TenMinuteTimeout     = 10 * time.Minute
	FifteenMinuteTimeout = 15 * time.Minute
	ThirtyMinuteTimeout  = 30 * time.Minute
)
