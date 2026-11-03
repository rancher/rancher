package clustermanager

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runConcurrently runs fs at the same time and waits for them, failing the test if they don't all return,
// which would mean a deadlock.
func runConcurrently(t *testing.T, fs ...func()) {
	t.Helper()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, f := range fs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			f()
		}()
	}
	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: concurrent calls did not return")
	}
}

func TestStopAndAFailedStartNeverLeaveAFailure(t *testing.T) {
	// Stop forgets the failure of a cluster stopped deliberately. A failed start of the record Stop stops
	// must not record its failure anyway, which would hold off the next start.
	for range 500 {
		m := &Manager{clusters: newTestClusters(newTestCluster("c-m-test", "uid-1"))}
		r := newStartedTestRecord("c-m-test", "uid-1", true)
		m.controllers.Store(r.clusterRec.UID, r)

		runConcurrently(t,
			func() { m.handleStartFailure(r, errors.New("boom")) },
			func() { m.Stop(r.clusterRec) },
		)

		require.Equal(t, ControllersState{Phase: ControllersStopped}, m.ControllersState(r.clusterRec))
	}
}

func TestStopWithErrorAndAStartThatSucceededNeverLoseTheFailure(t *testing.T) {
	// A start that succeeded only forgets failures while its record is current. Once stopWithError has
	// replaced it, the failure it recorded is the cluster's.
	for range 500 {
		m := &Manager{clusters: newTestClusters(newTestCluster("c-m-test", "uid-1"))}
		r := newStartedTestRecord("c-m-test", "uid-1", true)
		m.controllers.Store(r.clusterRec.UID, r)
		boom := errors.New("boom")

		runConcurrently(t,
			func() { m.handleStartSuccess(r) },
			func() { m.stopWithError(r.clusterRec, boom) },
		)

		state := m.ControllersState(r.clusterRec)
		require.Equal(t, ControllersFailed, state.Phase)
		require.Same(t, boom, state.Err)
	}
}

func TestFailuresAndRecordsCanBeUsedConcurrently(t *testing.T) {
	// Every path that touches failures, at once, for the same cluster: run with -race, and it must not
	// deadlock.
	for range 200 {
		m := &Manager{clusters: newTestClusters(newTestCluster("c-m-test", "uid-1"))}
		r := newStartedTestRecord("c-m-test", "uid-1", true)
		m.controllers.Store(r.clusterRec.UID, r)
		replacement := newStartedTestRecord("c-m-test", "uid-1", true)

		runConcurrently(t,
			func() { m.handleStartFailure(r, errors.New("boom")) },
			func() { m.handleStartSuccess(r) },
			func() { m.Stop(r.clusterRec) },
			func() { m.stopWithError(r.clusterRec, errors.New("unreachable")) },
			func() { _ = m.ControllersState(r.clusterRec) },
			func() {
				// As in start: a record is installed before its start can fail.
				if _, loaded := m.controllers.LoadOrStore(replacement.clusterRec.UID, replacement); !loaded {
					m.handleStartFailure(replacement, errors.New("boom again"))
				}
			},
			func() { _ = m.stopIfNotStarted(r) },
		)

		// Whatever the order, the cluster ends up in one consistent state.
		state := m.ControllersState(r.clusterRec)
		assert.Contains(t, []ControllersPhase{ControllersStopped, ControllersFailed, ControllersStarting}, state.Phase)
		if state.Phase == ControllersFailed {
			assert.Error(t, state.Err)
		}
	}
}
