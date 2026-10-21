package clustermanager

import (
	"context"
	"errors"
	"sync"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newMovedTestCluster returns a provisioned cluster whose connection details differ from those of the
// records built by newTestRecord. Records can't be built for it, since its service account token secret
// hasn't been migrated.
func newMovedTestCluster() *apimgmtv3.Cluster {
	cluster := newTestCluster("c-m-test", "uid-1")
	cluster.Status.APIEndpoint = "https://10.0.0.2"
	cluster.Status.CACert = "Y2E="
	apimgmtv3.ClusterConditionProvisioned.True(cluster)
	return cluster
}

func TestStartWithoutControllersKeepsRunningControllers(t *testing.T) {
	// A request finding the cluster's connection details changed must not tear its controllers down;
	// the sync replaces them, and is asked to.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	running := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(running.clusterRec.UID, running)

	r, err := m.start(context.Background(), newMovedTestCluster(), false, false)

	require.NoError(t, err)
	assert.Same(t, running, r, "the running record should keep serving until the sync replaces it")
	assert.NoError(t, running.ctx.Err(), "the controllers should be left running")
	assert.True(t, m.isActive(running))
	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters))
}

func TestStartWithoutControllersReplacesARecordWithoutControllers(t *testing.T) {
	m := &Manager{clusters: newTestClusters(newTestCluster("c-m-test", "uid-1"))}
	clientOnly := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(clientOnly.clusterRec.UID, clientOnly)
	cluster := newMovedTestCluster()

	_, err := m.start(context.Background(), cluster, false, false)

	require.Error(t, err, "the replacement can't be built")
	assert.Error(t, clientOnly.ctx.Err(), "the out of date record should be stopped")
	assert.False(t, m.isActive(clientOnly))
	assert.Equal(t, ControllersState{Phase: ControllersStopped}, m.ControllersState(cluster), "a request's failure is not the controllers'")
}

func TestStartWithoutControllersDoesNotRecordFailures(t *testing.T) {
	m := &Manager{clusters: newTestClusters(newTestCluster("c-m-test", "uid-1"))}
	cluster := newMovedTestCluster()

	_, err := m.start(context.Background(), cluster, false, false)

	require.Error(t, err)
	assert.Equal(t, ControllersState{Phase: ControllersStopped}, m.ControllersState(cluster))
}

func TestStopIfNotStartedLeavesStartedControllersAlone(t *testing.T) {
	m := &Manager{}
	r := newStartedTestRecord("c-m-test", "uid-1", true)
	m.controllers.Store(r.clusterRec.UID, r)

	assert.False(t, m.stopIfNotStarted(r))
	assert.NoError(t, r.ctx.Err())
	assert.True(t, m.isActive(r))
}

func TestStopIfNotStartedStopsARecordWithoutControllers(t *testing.T) {
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(r.clusterRec.UID, r)

	assert.True(t, m.stopIfNotStarted(r))
	assert.Error(t, r.ctx.Err())
	assert.False(t, m.isActive(r))
}

func TestStartControllerRefusesAStoppedRecord(t *testing.T) {
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")

	err := m.startController(r, true, true)

	assert.ErrorIs(t, err, errRecordReplaced)
	assert.False(t, r.started, "the controllers of a record that is gone must not be started")
}

func TestStopIfNotStartedAndStartControllerDoNotInterleave(t *testing.T) {
	// A request replacing a record without controllers can race the sync starting controllers on it.
	// Either the controllers start and the record stays, or the record goes and they never start.
	for range 200 {
		m := &Manager{clusters: newTestClusters(newTestCluster("c-m-test", "uid-1"))}
		r := newTestRecord("c-m-test", "uid-1")
		client := k8sfake.NewSimpleClientset()
		client.PrependReactor("get", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("unreachable")
		})
		r.cluster.K8sClient = client
		m.controllers.Store(r.clusterRec.UID, r)

		var (
			wg       sync.WaitGroup
			stopped  bool
			startErr error
		)
		wg.Add(2)
		go func() {
			defer wg.Done()
			stopped = m.stopIfNotStarted(r)
		}()
		go func() {
			defer wg.Done()
			startErr = m.startController(r, true, true)
		}()
		wg.Wait()

		r.Lock()
		started := r.started
		r.Unlock()
		if stopped {
			require.ErrorIs(t, startErr, errRecordReplaced)
			require.False(t, started)
		} else {
			require.NoError(t, startErr)
			require.True(t, started)
			require.True(t, m.isActive(r))
		}
		// Let the start that may be under way give up.
		r.cancel()
	}
}
