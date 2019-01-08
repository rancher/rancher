package kafka

import (
	"sync/atomic"
	"time"
)

// SummaryStats is a data structure that carries a summary of observed values.
// The average, minimum, and maximum are reported.
type SummaryStats struct {
	Avg int64 `metric:"avg" type:"gauge"`
	Min int64 `metric:"min" type:"gauge"`
	Max int64 `metric:"max" type:"gauge"`
}

// DurationStats is a data structure that carries a summary of observed duration
// values. The average, minimum, and maximum are reported.
type DurationStats struct {
	Avg time.Duration `metric:"avg" type:"gauge"`
	Min time.Duration `metric:"min" type:"gauge"`
	Max time.Duration `metric:"max" type:"gauge"`
}

// counter is an atomic incrementing counter which gets reset on snapshot.
//
// Since atomic is used to mutate the statistic the value must be 64-bit aligned.
// See https://golang.org/pkg/sync/atomic/#pkg-note-BUG
type counter int64

func (c *counter) ptr() *int64 {
	return (*int64)(c)
}

func (c *counter) observe(v int64) {
	atomic.AddInt64(c.ptr(), v)
}

func (c *counter) snapshot() int64 {
	p := c.ptr()
	v := atomic.LoadInt64(p)
	atomic.AddInt64(p, -v)
	return v
}

// gauge is an atomic integer that may be set to any arbitrary value, the value
// does not change after a snapshot.
//
// Since atomic is used to mutate the statistic the value must be 64-bit aligned.
// See https://golang.org/pkg/sync/atomic/#pkg-note-BUG
type gauge int64

func (g *gauge) ptr() *int64 {
	return (*int64)(g)
}

func (g *gauge) observe(v int64) {
	atomic.StoreInt64(g.ptr(), v)
}

func (g *gauge) snapshot() int64 {
	return atomic.LoadInt64(g.ptr())
}

// minimum is an atomic integral type that keeps track of the minimum of all
// values that it observed between snapshots.
//
// Since atomic is used to mutate the statistic the value must be 64-bit aligned.
// See https://golang.org/pkg/sync/atomic/#pkg-note-BUG
type minimum int64

func (m *minimum) ptr() *int64 {
	return (*int64)(m)
}

func (m *minimum) observe(v int64) {
	for {
		ptr := m.ptr()
		min := atomic.LoadInt64(ptr)

		if min >= 0 && min <= v {
			break
		}

		if atomic.CompareAndSwapInt64(ptr, min, v) {
			break
		}
	}
}

func (m *minimum) snapshot() int64 {
	p := m.ptr()
	v := atomic.LoadInt64(p)
	atomic.CompareAndSwapInt64(p, v, -1)
	if v < 0 {
		v = 0
	}
	return v
}

// maximum is an atomic integral type that keeps track of the maximum of all
// values that it observed between snapshots.
//
// Since atomic is used to mutate the statistic the value must be 64-bit aligned.
// See https://golang.org/pkg/sync/atomic/#pkg-note-BUG
type maximum int64

func (m *maximum) ptr() *int64 {
	return (*int64)(m)
}

func (m *maximum) observe(v int64) {
	for {
		ptr := m.ptr()
		max := atomic.LoadInt64(ptr)

		if max >= 0 && max >= v {
			break
		}

		if atomic.CompareAndSwapInt64(ptr, max, v) {
			break
		}
	}
}

func (m *maximum) snapshot() int64 {
	p := m.ptr()
	v := atomic.LoadInt64(p)
	atomic.CompareAndSwapInt64(p, v, -1)
	if v < 0 {
		v = 0
	}
	return v
}

type summary struct {
	min   minimum
	max   maximum
	sum   counter
	count counter
}

func makeSummary() summary {
	return summary{
		min: -1,
		max: -1,
	}
}

func (s *summary) observe(v int64) {
	s.min.observe(v)
	s.max.observe(v)
	s.sum.observe(v)
	s.count.observe(1)
}

func (s *summary) observeDuration(v time.Duration) {
	s.observe(int64(v))
}

func (s *summary) snapshot() SummaryStats {
	avg := int64(0)
	min := s.min.snapshot()
	max := s.max.snapshot()
	sum := s.sum.snapshot()
	count := s.count.snapshot()

	if count != 0 {
		avg = int64(float64(sum) / float64(count))
	}

	return SummaryStats{
		Avg: avg,
		Min: min,
		Max: max,
	}
}

func (s *summary) snapshotDuration() DurationStats {
	summary := s.snapshot()
	return DurationStats{
		Avg: time.Duration(summary.Avg),
		Min: time.Duration(summary.Min),
		Max: time.Duration(summary.Max),
	}
}
