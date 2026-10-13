package config

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

// useTestRetry shortens the deferred start backoff so the tests don't wait on the real one.
func useTestRetry(t *testing.T, steps int) {
	t.Helper()
	original := deferredStartRetry
	deferredStartRetry = wait.Backoff{Steps: steps, Duration: time.Millisecond, Factor: 1}
	t.Cleanup(func() { deferredStartRetry = original })
}

func TestDeferredStartRetriesUntilItSucceeds(t *testing.T) {
	useTestRetry(t, 5)

	var calls atomic.Int32
	done := make(chan struct{})
	w := &UserContext{
		ClusterName: "c-m-test",
		OnDeferredStartError: func() {
			t.Error("OnDeferredStartError called for a start that eventually succeeded")
		},
	}

	starter := w.deferredStart(context.Background(), func() error {
		if calls.Add(1) < 3 {
			return errors.New("apiserver is unreachable")
		}
		close(done)
		return nil
	})

	starter()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the deferred start to succeed")
	}
	assert.EqualValues(t, 3, calls.Load())
}

func TestDeferredStartEscalatesWhenItKeepsFailing(t *testing.T) {
	const steps = 3
	useTestRetry(t, steps)

	var calls atomic.Int32
	failed := make(chan struct{})
	w := &UserContext{
		ClusterName:          "c-m-test",
		OnDeferredStartError: func() { close(failed) },
	}

	starter := w.deferredStart(context.Background(), func() error {
		calls.Add(1)
		return errors.New("apiserver is unreachable")
	})

	starter()

	select {
	case <-failed:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for OnDeferredStartError")
	}
	assert.EqualValues(t, steps, calls.Load(), "register should be retried until the backoff is exhausted")
}

func TestDeferredStartEscalationIsOptional(t *testing.T) {
	useTestRetry(t, 1)

	called := make(chan struct{})
	// OnDeferredStartError is left unset: clustermanager sets it, but other callers may not.
	w := &UserContext{ClusterName: "c-m-test"}

	starter := w.deferredStart(context.Background(), func() error {
		close(called)
		return errors.New("apiserver is unreachable")
	})

	starter()

	select {
	case <-called:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the deferred start to run")
	}
}

func TestDeferredStartRunsOnce(t *testing.T) {
	useTestRetry(t, 5)

	// The starter is called from controller handlers, so it fires repeatedly: while an earlier
	// attempt is still retrying, and again long after one succeeded. Neither may start a second run.
	release := make(chan struct{})
	var concurrent, peak, calls atomic.Int32
	finished := make(chan struct{})

	w := &UserContext{ClusterName: "c-m-test"}
	starter := w.deferredStart(context.Background(), func() error {
		if in := concurrent.Add(1); in > peak.Load() {
			peak.Store(in)
		}
		defer concurrent.Add(-1)

		switch calls.Add(1) {
		case 1:
			<-release
			return errors.New("apiserver is unreachable")
		case 2:
			close(finished)
		}
		return nil
	})

	starter()
	// Wait for the first attempt to be in flight before piling on.
	require.Eventually(t, func() bool { return calls.Load() == 1 }, 10*time.Second, time.Millisecond)

	for range 10 {
		starter()
	}
	close(release)

	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the deferred start to succeed")
	}
	assert.EqualValues(t, 1, peak.Load(), "only one attempt should run at a time")

	for range 10 {
		starter()
	}
	assert.Never(t, func() bool { return calls.Load() > 2 }, 100*time.Millisecond, 10*time.Millisecond,
		"the starter should do nothing once the controllers are up")
}

func TestDeferredStartStopsWhenTheContextIsCancelled(t *testing.T) {
	useTestRetry(t, 100)

	ctx, cancel := context.WithCancel(context.Background())
	running := make(chan struct{})
	var once, calls atomic.Int32
	w := &UserContext{
		ClusterName: "c-m-test",
		OnDeferredStartError: func() {
			t.Error("OnDeferredStartError called after the context was cancelled")
		},
	}

	starter := w.deferredStart(ctx, func() error {
		calls.Add(1)
		if once.Add(1) == 1 {
			close(running)
		}
		return errors.New("apiserver is unreachable")
	})

	starter()
	<-running
	cancel()

	// The loop should stop on its own rather than run out its 100 steps.
	require.Eventually(t, func() bool {
		before := calls.Load()
		time.Sleep(50 * time.Millisecond)
		return calls.Load() == before
	}, 10*time.Second, 50*time.Millisecond)
}
