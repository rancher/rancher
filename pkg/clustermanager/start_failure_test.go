package clustermanager

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/dialer"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// newTestClusters returns a cluster client backed by a single stored cluster, or none if stored is nil.
func newTestClusters(stored *apimgmtv3.Cluster) *fakes.ClusterInterfaceMock {
	return &fakes.ClusterInterfaceMock{
		GetFunc: func(name string, _ metav1.GetOptions) (*apimgmtv3.Cluster, error) {
			if stored == nil || stored.Name != name {
				return nil, apierrors.NewNotFound(schema.GroupResource{Group: "management.cattle.io", Resource: "clusters"}, name)
			}
			return stored.DeepCopy(), nil
		},
		UpdateStatusFunc: func(c *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
			stored = c.DeepCopy()
			return c, nil
		},
	}
}

func newTestCluster(name string, uid types.UID) *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name, UID: uid}}
}

func TestMarkNotReadyMarksTheCluster(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}

	m.markNotReady(newTestCluster("c-m-test", "uid-1"), reasonAPIServerUnreachable, errors.New("boom"))

	require.Len(t, clusters.UpdateStatusCalls(), 1)
	updated := clusters.UpdateStatusCalls()[0].In1
	assert.True(t, apimgmtv3.ClusterConditionReady.IsFalse(updated))
	assert.Equal(t, reasonAPIServerUnreachable, apimgmtv3.ClusterConditionReady.GetReason(updated))
	assert.Equal(t, "boom", apimgmtv3.ClusterConditionReady.GetMessage(updated))
}

func TestMarkNotReadyIgnoresAClusterThatWasReplaced(t *testing.T) {
	// A cluster deleted and recreated under the same name is a different cluster; a failure of the old
	// one must not be reported on it.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-2"))
	m := &Manager{clusters: clusters}

	m.markNotReady(newTestCluster("c-m-test", "uid-1"), reasonUserControllersFailed, errors.New("boom"))

	assert.Empty(t, clusters.UpdateStatusCalls())
}

func TestMarkNotReadyIgnoresAClusterThatIsGone(t *testing.T) {
	clusters := newTestClusters(nil)
	m := &Manager{clusters: clusters}

	m.markNotReady(newTestCluster("c-m-test", "uid-1"), reasonUserControllersFailed, errors.New("boom"))

	assert.Empty(t, clusters.UpdateStatusCalls())
}

func TestMarkNotReadySkipsTheUpdateWhenNothingChanged(t *testing.T) {
	stored := newTestCluster("c-m-test", "uid-1")
	apimgmtv3.ClusterConditionReady.False(stored)
	apimgmtv3.ClusterConditionReady.Reason(stored, reasonUserControllersFailed)
	apimgmtv3.ClusterConditionReady.Message(stored, "boom")
	clusters := newTestClusters(stored)
	m := &Manager{clusters: clusters}

	m.markNotReady(newTestCluster("c-m-test", "uid-1"), reasonUserControllersFailed, errors.New("boom"))

	assert.Empty(t, clusters.UpdateStatusCalls())
}

func TestHandleStartFailureMarksTheClusterAndStopsTheRecord(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	r := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(r.clusterRec.UID, r)

	m.handleStartFailure(r, fmt.Errorf("%w: dial failed", errAPIServerUnreachable))

	require.Len(t, clusters.UpdateStatusCalls(), 1)
	assert.Equal(t, reasonAPIServerUnreachable, apimgmtv3.ClusterConditionReady.GetReason(clusters.UpdateStatusCalls()[0].In1))
	assert.Error(t, r.ctx.Err(), "the failed record should be cancelled")
	_, ok := m.controllers.Load(r.clusterRec.UID)
	assert.False(t, ok, "the failed record should be removed so it is built again")
}

func TestHandleStartFailureLeavesTheReplacementRunning(t *testing.T) {
	// A record replaced while it was starting is cancelled, and its start fails with that. Reporting it
	// is fine, but stopping the cluster on its behalf would tear down the replacement.
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	stale := newTestRecord("c-m-test", "uid-1")
	stale.cancel()
	current := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(current.clusterRec.UID, current)

	m.handleStartFailure(stale, stale.ctx.Err())

	require.Len(t, clusters.UpdateStatusCalls(), 1)
	assert.Equal(t, reasonUserControllersRestarting, apimgmtv3.ClusterConditionReady.GetReason(clusters.UpdateStatusCalls()[0].In1))
	assert.NoError(t, current.ctx.Err(), "the replacement should be left running")
	obj, ok := m.controllers.Load(current.clusterRec.UID)
	require.True(t, ok, "the replacement should still be registered")
	assert.Same(t, current, obj)
}

func TestHandleStartFailureDoesNotTouchANewClusterWithTheSameName(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-2"))
	m := &Manager{clusters: clusters}
	old := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(old.clusterRec.UID, old)
	current := newTestRecord("c-m-test", "uid-2")
	m.controllers.Store(current.clusterRec.UID, current)

	m.handleStartFailure(old, errors.New("boom"))

	assert.Empty(t, clusters.UpdateStatusCalls(), "the new cluster must not be marked")
	assert.NoError(t, current.ctx.Err(), "the new cluster's record should be left running")
	_, ok := m.controllers.Load(old.clusterRec.UID)
	assert.False(t, ok, "the old record should be removed")
}

func TestHandleStartFailureDoesNotReportADisconnectedAgent(t *testing.T) {
	clusters := newTestClusters(newTestCluster("c-m-test", "uid-1"))
	m := &Manager{clusters: clusters}
	r := newTestRecord("c-m-test", "uid-1")
	m.controllers.Store(r.clusterRec.UID, r)

	m.handleStartFailure(r, &url.Error{Op: "Get", URL: "https://10.0.0.1", Err: dialer.ErrAgentDisconnected})

	assert.Empty(t, clusters.UpdateStatusCalls(), "clusterconnected reports a disconnected agent")
	_, ok := m.controllers.Load(r.clusterRec.UID)
	assert.False(t, ok, "the record should still be stopped")
}

func TestClassifyStartError(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	running := context.Background()

	tests := []struct {
		name       string
		ctx        context.Context
		err        error
		wantReason string
		wantReport bool
	}{
		{
			name: "agent disconnected is left to clusterconnected",
			ctx:  running,
			err:  &url.Error{Op: "Get", URL: "https://10.0.0.1", Err: dialer.ErrAgentDisconnected},
		},
		{
			name:       "sync timeout is a failure even though the record cancelled itself",
			ctx:        cancelled,
			err:        errControllersSyncTimeout,
			wantReason: reasonUserControllersFailed,
			wantReport: true,
		},
		{
			name:       "unreachable probe",
			ctx:        running,
			err:        fmt.Errorf("%w: %w", errAPIServerUnreachable, errors.New("eof")),
			wantReason: reasonAPIServerUnreachable,
			wantReport: true,
		},
		{
			name:       "stopped record",
			ctx:        cancelled,
			err:        context.Canceled,
			wantReason: reasonUserControllersRestarting,
			wantReport: true,
		},
		{
			name:       "dial error",
			ctx:        running,
			err:        &url.Error{Op: "Get", URL: "https://10.0.0.1", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}},
			wantReason: reasonAPIServerUnreachable,
			wantReport: true,
		},
		{
			name:       "deadline exceeded",
			ctx:        running,
			err:        fmt.Errorf("list namespaces: %w", context.DeadlineExceeded),
			wantReason: reasonAPIServerUnreachable,
			wantReport: true,
		},
		{
			name:       "service unavailable",
			ctx:        running,
			err:        apierrors.NewServiceUnavailable("etcd is down"),
			wantReason: reasonAPIServerUnreachable,
			wantReport: true,
		},
		{
			name: "certificate that doesn't verify",
			ctx:  running,
			err: &url.Error{Op: "Get", URL: "https://10.0.0.1", Err: &tls.CertificateVerificationError{
				Err: x509.UnknownAuthorityError{},
			}},
			wantReason: reasonUserControllersFailed,
			wantReport: true,
		},
		{
			name:       "conflict",
			ctx:        running,
			err:        apierrors.NewConflict(schema.GroupResource{Resource: "clusters"}, "c-m-test", errors.New("modified")),
			wantReason: reasonUserControllersFailed,
			wantReport: true,
		},
		{
			name:       "anything else",
			ctx:        running,
			err:        errors.New("boom"),
			wantReason: reasonUserControllersFailed,
			wantReport: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, report := classifyStartError(tt.ctx, tt.err)
			assert.Equal(t, tt.wantReport, report)
			assert.Equal(t, tt.wantReason, reason)
		})
	}
}
