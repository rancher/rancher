package defaults

import "time"

var (
	WatchTimeoutSeconds           = int64(60 * 30) // 30 minutes.
	FiveHundredMillisecondTimeout = 500 * time.Millisecond
	FiveSecondTimeout             = 5 * time.Second
	TenSecondTimeout              = 10 * time.Second
	OneMinuteTimeout              = 1 * time.Minute
	TwoMinuteTimeout              = 2 * time.Minute
	FiveMinuteTimeout             = 5 * time.Minute
	TenMinuteTimeout              = 10 * time.Minute
	FifteenMinuteTimeout          = 15 * time.Minute
	ThirtyMinuteTimeout           = 30 * time.Minute
)
