package clustermanager

import (
	"context"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecordKeepsItsOwnCopyOfTheCluster(t *testing.T) {
	// The cluster comes from the informer's cache, which must never be changed through the record, nor
	// change the record under the feet of its controllers.
	m := &Manager{}
	cached := newMovedTestCluster()

	r := m.newRecord(context.Background(), cached, &config.UserContext{ClusterName: cached.Name})

	require.NotSame(t, cached, r.clusterRec)
	assert.Equal(t, cached, r.clusterRec)

	cached.Status.APIEndpoint = "https://10.0.0.3"
	apimgmtv3.ClusterConditionPreBootstrapped.True(cached)
	assert.Equal(t, "https://10.0.0.2", r.clusterRec.Status.APIEndpoint)
	assert.False(t, apimgmtv3.ClusterConditionPreBootstrapped.IsTrue(r.clusterRec))

	apimgmtv3.ClusterConditionControllersRunning.True(r.clusterRec)
	assert.Empty(t, apimgmtv3.ClusterConditionControllersRunning.GetStatus(cached))
}

func TestNewRecordReportsDeferredStartFailures(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	clusterContext := &config.UserContext{ClusterName: "c-m-test"}
	r := m.newRecord(context.Background(), newTestCluster("c-m-test", "uid-1"), clusterContext)
	m.controllers.Store(r.clusterRec.UID, r)
	r.started, r.phase = true, ControllersRunning

	before := time.Now()
	clusterContext.OnDeferredStartError()

	assert.Error(t, r.ctx.Err(), "the record should be dropped so the next start builds a new one")
	assert.False(t, m.isActive(r))
	state := m.ControllersState(r.clusterRec)
	assert.Equal(t, ControllersFailed, state.Phase)
	assert.ErrorIs(t, state.Err, errDeferredStart)
	assert.WithinRange(t, state.RetryAt, before.Add(startRetryBaseDelay), time.Now().Add(startRetryBaseDelay), "it is retried like any failed start")
	assert.Equal(t, []string{"c-m-test"}, enqueued(clusters))
}
