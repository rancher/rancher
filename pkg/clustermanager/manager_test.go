package clustermanager

import (
	"context"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newTestRecord(clusterName string, uid types.UID) *record {
	r := &record{
		cluster:    &config.UserContext{ClusterName: clusterName},
		clusterRec: &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName, UID: uid}},
	}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r
}

func TestStopRecordStopsTheActiveRecord(t *testing.T) {
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(r.clusterRec.UID, r)

	m.stopRecord(r)

	assert.Error(t, r.ctx.Err(), "the record context should be cancelled")
	_, ok := m.controllers.Load(r.clusterRec.UID)
	assert.False(t, ok, "the record should be removed so it is built again")
}

func TestStopRecordIgnoresARecordThatWasAlreadyReplaced(t *testing.T) {
	// A record's callbacks can outlive it. Stopping the cluster on behalf of a stale record would
	// tear down controllers that are running fine.
	m := &Manager{}
	stale := newTestRecord("c-m-test", "uid-1")
	current := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(current.clusterRec.UID, current)

	m.stopRecord(stale)

	assert.NoError(t, current.ctx.Err(), "the current record should be left running")
	obj, ok := m.controllers.Load(current.clusterRec.UID)
	require.True(t, ok, "the current record should still be registered")
	assert.Same(t, current, obj)
}

func TestStopRecordIsANoopWhenTheClusterIsGone(t *testing.T) {
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")

	assert.NotPanics(t, func() { m.stopRecord(r) })
}

func TestStopStopsTheRecordThatIsActuallyRegistered(t *testing.T) {
	// Stop resolves the cluster to whatever record is current rather than trusting the one its
	// caller happens to be holding.
	m := &Manager{}
	stale := newTestRecord("c-m-test", "uid-1")
	current := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(current.clusterRec.UID, current)

	m.Stop(stale.clusterRec)

	assert.NoError(t, stale.ctx.Err(), "the caller's stale record is not what Stop acts on")
	assert.Error(t, current.ctx.Err(), "the registered record should be stopped")
	_, ok := m.controllers.Load(current.clusterRec.UID)
	assert.False(t, ok, "the cluster should have no record left")
}

func TestStopClearsTheClusterWhileRecordsAreBeingReplaced(t *testing.T) {
	// The entry can change between Stop's load and its delete. Whichever call wins the delete is the
	// one that cancels, so no record is left running and none is cancelled twice.
	m := &Manager{}
	const uid = types.UID("uid-1")
	clusterRec := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test", UID: uid}}

	replaced := make(chan struct{})
	go func() {
		defer close(replaced)
		for range 500 {
			m.controllers.Store(uid, newTestRecord("c-m-test", uid))
		}
	}()

	for range 500 {
		m.Stop(clusterRec)
	}
	<-replaced
	m.Stop(clusterRec)

	_, ok := m.controllers.Load(uid)
	assert.False(t, ok, "Stop should leave no record behind once nothing else is installing one")
}

func TestStopIsANoopWhenTheClusterIsGone(t *testing.T) {
	m := &Manager{}
	r := newTestRecord("c-m-test", "uid-1")

	assert.NotPanics(t, func() { m.Stop(r.clusterRec) })
}
