// Copyright 2015 Marc-Antoine Ruel. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package stack

import (
	"sort"
)

// Similarity is the level at which two call lines arguments must match to be
// considered similar enough to coalesce them.
type Similarity int

const (
	// ExactFlags requires same bits (e.g. Locked).
	ExactFlags Similarity = iota
	// ExactLines requests the exact same arguments on the call line.
	ExactLines
	// AnyPointer considers different pointers a similar call line.
	AnyPointer
	// AnyValue accepts any value as similar call line.
	AnyValue
)

// Bucketize returns the number of similar goroutines.
func Bucketize(goroutines []Goroutine, similar Similarity) map[*Signature][]Goroutine {
	out := map[*Signature][]Goroutine{}
	// O(n²). Fix eventually.
	for _, routine := range goroutines {
		found := false
		for key := range out {
			// When a match is found, this effectively drops the other goroutine ID.
			if key.Similar(&routine.Signature, similar) {
				found = true
				if !key.Equal(&routine.Signature) {
					// Almost but not quite equal. There's different pointers passed
					// around but the same values. Zap out the different values.
					newKey := key.Merge(&routine.Signature)
					out[newKey] = append(out[key], routine)
					delete(out, key)
				} else {
					out[key] = append(out[key], routine)
				}
				break
			}
		}
		if !found {
			key := &Signature{}
			*key = routine.Signature
			out[key] = []Goroutine{routine}
		}
	}
	return out
}

// Bucket is a stack trace signature and the list of goroutines that fits this
// signature.
type Bucket struct {
	Signature
	Routines []Goroutine
}

// First returns true if it contains the first goroutine, e.g. the ones that
// likely generated the panic() call, if any.
func (b *Bucket) First() bool {
	for _, r := range b.Routines {
		if r.First {
			return true
		}
	}
	return false
}

// Less does reverse sort.
func (b *Bucket) Less(r *Bucket) bool {
	if b.First() {
		return true
	}
	if r.First() {
		return false
	}
	return b.Signature.Less(&r.Signature)
}

// Buckets is a list of Bucket sorted by repeation count.
type Buckets []Bucket

func (b Buckets) Len() int {
	return len(b)
}

func (b Buckets) Less(i, j int) bool {
	return b[i].Less(&b[j])
}

func (b Buckets) Swap(i, j int) {
	b[j], b[i] = b[i], b[j]
}

// SortBuckets creates a list of Bucket from each goroutine stack trace count.
func SortBuckets(buckets map[*Signature][]Goroutine) Buckets {
	out := make(Buckets, 0, len(buckets))
	for signature, count := range buckets {
		out = append(out, Bucket{*signature, count})
	}
	sort.Sort(out)
	return out
}
