package kafka

import (
	"math"
	"time"
)

const (
	maxTimeout = time.Duration(math.MaxInt32) * time.Millisecond
	minTimeout = time.Duration(math.MinInt32) * time.Millisecond
	defaultRTT = 1 * time.Second
)

func timestamp(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond)
}

func timestampToTime(t int64) time.Time {
	return time.Unix(t/1000, (t%1000)*int64(time.Millisecond))
}

func duration(ms int32) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

func milliseconds(d time.Duration) int32 {
	switch {
	case d > maxTimeout:
		d = maxTimeout
	case d < minTimeout:
		d = minTimeout
	}
	return int32(d / time.Millisecond)
}

func deadlineToTimeout(deadline time.Time, now time.Time) time.Duration {
	if deadline.IsZero() {
		return maxTimeout
	}
	return deadline.Sub(now)
}

func adjustDeadlineForRTT(deadline time.Time, now time.Time, rtt time.Duration) time.Time {
	if !deadline.IsZero() {
		timeout := deadline.Sub(now)
		if timeout < rtt {
			rtt = timeout / 4
		}
		deadline = deadline.Add(-rtt)
	}
	return deadline
}
