package clustermanager

import (
	"errors"
	"net/url"
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/dialer"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3/fakes"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// newTestClusters returns a cluster client backed by a single stored cluster, or none if stored is nil.
// Its controller records the clusters that are enqueued, see enqueued.
func newTestClusters(stored *apimgmtv3.Cluster) *fakes.ClusterInterfaceMock {
	controller := &fakes.ClusterControllerMock{
		EnqueueFunc: func(string, string) {},
	}
	return &fakes.ClusterInterfaceMock{
		ControllerFunc: func() v3.ClusterController { return controller },
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

// enqueued returns the names of the clusters enqueued through clusters' controller.
func enqueued(clusters *fakes.ClusterInterfaceMock) []string {
	var names []string
	for _, call := range clusters.Controller().(*fakes.ClusterControllerMock).EnqueueCalls() {
		names = append(names, call.Name)
	}
	return names
}

func newTestCluster(name string, uid types.UID) *apimgmtv3.Cluster {
	return &apimgmtv3.Cluster{Name: name, UID: uid}
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

	_, ok := m.controllers.Load(r.clusterRec.UID)
	assert.False(t, ok, "the record should still be stopped")
}

func TestStartAllowed(t *testing.T) {
	now := metav1.Now()
	removing := func(uninstall string) *apimgmtv3.Cluster {
		c := &apimgmtv3.Cluster{Name: "c-m-test", DeletionTimestamp: &now}
		if uninstall != "" {
			apimgmtv3.ClusterConditionAgentUninstallScheduled.SetStatus(c, uninstall)
		}
		return c
	}

	assert.True(t, startAllowed(newTestCluster("c-m-test", "uid-1"), true), "controllers of a live cluster")
	assert.True(t, startAllowed(newTestCluster("c-m-test", "uid-1"), false), "clients of a live cluster")

	assert.False(t, startAllowed(removing(""), true), "controllers are never started for a cluster being removed")
	assert.True(t, startAllowed(removing(""), false), "clients are available while the removal still cleans up through the downstream cluster")
	assert.True(t, startAllowed(removing("Unknown"), false))
	assert.False(t, startAllowed(removing("True"), false), "not once the agent uninstall is recorded")
	assert.False(t, startAllowed(removing("False"), false), "not once the agent uninstall gave up either")
}
