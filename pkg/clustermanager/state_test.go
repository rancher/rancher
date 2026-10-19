package clustermanager

import (
	"context"
	"errors"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newStartedTestRecord returns a record whose controllers were asked to start, as startController leaves it.
func newStartedTestRecord(clusterName string, uid types.UID, owner bool) *record {
	r := newTestRecord(clusterName, uid)
	r.started = true
	r.owner = owner
	r.phase = ControllersStarting
	return r
}

func TestControllersStateOfAClusterWithoutARecord(t *testing.T) {
	m := &Manager{}

	state := m.ControllersState(newTestCluster("c-m-test", "uid-1"))

	assert.Equal(t, ControllersState{Phase: ControllersStopped}, state)
}

func TestControllersStateOfARecordWithoutControllers(t *testing.T) {
	// API requests build records that only serve clients; no controllers run for them.
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(r.clusterRec.UID, r)

	assert.Equal(t, ControllersStopped, m.ControllersState(r.clusterRec).Phase)
}

func TestControllersStateOfAStartingRecord(t *testing.T) {
	m := &Manager{}
	r := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(r.clusterRec.UID, r)

	assert.Equal(t, ControllersState{Phase: ControllersStarting, Owner: true}, m.ControllersState(r.clusterRec))
}

func TestControllersStateReportsAFailureOnceTheRecordIsGone(t *testing.T) {
	m := &Manager{}
	cluster := newTestCluster("c-m-test", "uid-1")
	boom := errors.New("boom")
	m.failures.Store(cluster.UID, boom)

	state := m.ControllersState(cluster)

	assert.Equal(t, ControllersFailed, state.Phase)
	assert.Same(t, boom, state.Err)
}

func TestControllersStatePrefersTheCurrentRecordOverAnEarlierFailure(t *testing.T) {
	m := &Manager{}
	r := newStartedTestRecord("c-m-test", "uid-1", false)
	m.controllers.Store(r.clusterRec.UID, r)
	m.failures.Store(r.clusterRec.UID, errors.New("boom"))

	assert.Equal(t, ControllersStarting, m.ControllersState(r.clusterRec).Phase)
}

func TestStopForgetsAFailure(t *testing.T) {
	m := &Manager{}
	cluster := newTestCluster("c-m-test", "uid-1")
	m.failures.Store(cluster.UID, errors.New("boom"))

	m.Stop(cluster)

	assert.Equal(t, ControllersStopped, m.ControllersState(cluster).Phase)
}

func TestStopWithErrorStopsAndRecordsTheFailure(t *testing.T) {
	// Stop forgets failures, so the failure has to survive the stop it comes with.
	m := &Manager{}
	r := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(r.clusterRec.UID, r)
	boom := errors.New("boom")

	m.stopWithError(r.clusterRec, boom)

	assert.Error(t, r.ctx.Err(), "the record should be stopped")
	state := m.ControllersState(r.clusterRec)
	assert.Equal(t, ControllersFailed, state.Phase)
	assert.Same(t, boom, state.Err)
}

func TestHandleStartSuccessRecordsRunningAndEnqueues(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	r := newStartedTestRecord("c-m-test", "uid-1", false)
	m.controllers.Store(r.clusterRec.UID, r)
	m.failures.Store(r.clusterRec.UID, errors.New("an earlier start failed"))

	m.handleStartSuccess(r)

	state := m.ControllersState(r.clusterRec)
	assert.Equal(t, ControllersRunning, state.Phase)
	assert.NoError(t, state.Err)
	_, failed := m.failures.Load(r.clusterRec.UID)
	assert.False(t, failed, "the earlier failure should be forgotten")
	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters))
	assert.Empty(t, clusters.UpdateStatusCalls(), "the cluster is only written by its handlers")
}

func TestHandleStartSuccessDoesNotWriteTheOwnersCluster(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	r := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(r.clusterRec.UID, r)

	m.handleStartSuccess(r)

	assert.Empty(t, clusters.UpdateStatusCalls(), "the cluster is only written by its handlers")
	assert.Equal(t, ControllersState{Phase: ControllersRunning, Owner: true}, m.ControllersState(r.clusterRec))
	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters))
}

func TestHandleStartSuccessIgnoresARecordThatWasReplaced(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	stale := newStartedTestRecord("c-m-test", "uid-1", true)
	current := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(current.clusterRec.UID, current)

	m.handleStartSuccess(stale)

	assert.Equal(t, ControllersStarting, m.ControllersState(current.clusterRec).Phase)
	assert.Empty(t, clusters.UpdateStatusCalls())
	assert.Empty(t, enqueued(clusters))
}

func TestHandleStartFailureRecordsTheFailureAndEnqueues(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	r := newStartedTestRecord("c-m-test", "uid-1", false)
	m.controllers.Store(r.clusterRec.UID, r)
	boom := errors.New("boom")

	m.handleStartFailure(r, boom)

	assert.Error(t, r.ctx.Err(), "the record should be stopped")
	state := m.ControllersState(r.clusterRec)
	assert.Equal(t, ControllersFailed, state.Phase)
	assert.Same(t, boom, state.Err)
	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters))
	assert.Empty(t, clusters.UpdateStatusCalls(), "the cluster is only written by its handlers")
}

func TestHandleStartFailureDoesNotWriteTheOwnersCluster(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	r := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(r.clusterRec.UID, r)

	m.handleStartFailure(r, errors.New("boom"))

	assert.Empty(t, clusters.UpdateStatusCalls(), "the cluster is only written by its handlers")
	assert.Equal(t, ControllersFailed, m.ControllersState(r.clusterRec).Phase)
}

func TestHandleStartFailureOnlyEnqueuesWhenTheClusterStartsFailing(t *testing.T) {
	// The sync an enqueue triggers starts the controllers again; enqueueing on every failure would retry
	// a cluster that keeps failing without any delay.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}

	first := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(first.clusterRec.UID, first)
	m.handleStartFailure(first, errors.New("boom"))

	second := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(second.clusterRec.UID, second)
	again := errors.New("boom again")
	m.handleStartFailure(second, again)

	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters), "only the first failure should enqueue")
	assert.Same(t, again, m.ControllersState(second.clusterRec).Err, "the latest failure should be reported")

	third := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(third.clusterRec.UID, third)
	m.handleStartSuccess(third)
	fourth := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(fourth.clusterRec.UID, fourth)
	m.handleStartFailure(fourth, errors.New("boom"))

	assert.Equal(t, []string{"c-m-test", "c-m-test", "c-m-test"}, enqueued(clusters), "failing again after a success should enqueue")
}

func TestStartRecordsAFailureToBuildTheRecord(t *testing.T) {
	// The cluster can't be connected to until its service account token secret is migrated.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	cluster := newTestCluster("c-m-test", "uid-1")
	cluster.Status.APIEndpoint = "https://10.0.0.1"
	cluster.Status.CACert = "Y2E="
	apimgmtv3.ClusterConditionProvisioned.True(cluster)
	running := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(running.clusterRec.UID, running)

	_, err := m.start(context.Background(), cluster, true, true)

	require.Error(t, err)
	assert.Error(t, running.ctx.Err(), "the record for the old connection details should be stopped")
	state := m.ControllersState(cluster)
	assert.Equal(t, ControllersFailed, state.Phase)
	assert.Equal(t, err, state.Err)
	assert.Empty(t, clusters.UpdateStatusCalls(), "the cluster is only written by its handlers")
	assert.Empty(t, enqueued(clusters), "the sync that failed to start the controllers reports it")
}

func TestHandleStartFailureIgnoresARecordThatWasReplaced(t *testing.T) {
	// The failure of a record that was already replaced is not the cluster's: the replacement may be
	// running fine.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	stale := newStartedTestRecord("c-m-test", "uid-1", true)
	current := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(current.clusterRec.UID, current)

	m.handleStartFailure(stale, errors.New("boom"))

	assert.NoError(t, current.ctx.Err(), "the current record should be left running")
	assert.Equal(t, ControllersStarting, m.ControllersState(current.clusterRec).Phase)
	_, failed := m.failures.Load(current.clusterRec.UID)
	assert.False(t, failed)
	assert.Empty(t, clusters.UpdateStatusCalls())
	assert.Empty(t, enqueued(clusters))
}

func TestStartControllerReportsAFailedStart(t *testing.T) {
	// Exercises the whole path from startController to the failure being reported: the downstream
	// cluster can't be reached and the record is stopped while waiting to retry.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters, startSem: semaphore.NewWeighted(1)}
	r := newTestRecord("c-m-test", "uid-1")
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("get", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("unreachable")
	})
	r.cluster.K8sClient = client
	m.controllers.Store(r.clusterRec.UID, r)
	r.cancel()

	require.NoError(t, m.startController(r, true, false))

	assert.Eventually(t, func() bool {
		return m.ControllersState(r.clusterRec).Phase == ControllersFailed
	}, 5*time.Second, 10*time.Millisecond)
	assert.ErrorIs(t, m.ControllersState(r.clusterRec).Err, context.Canceled)
	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters))
}

func TestStartControllerWithoutControllersDoesNotStartThem(t *testing.T) {
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(r.clusterRec.UID, r)

	require.NoError(t, m.startController(r, false, true))

	assert.Equal(t, ControllersStopped, m.ControllersState(r.clusterRec).Phase)
}
