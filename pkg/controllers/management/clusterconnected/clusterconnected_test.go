package clusterconnected

import (
	"testing"

	"github.com/rancher/rancher/pkg/api/steve/proxy"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeSessions records which keys were probed and reports a session for a fixed set.
type fakeSessions struct {
	present map[string]bool
	asked   []string
}

func (f *fakeSessions) HasSession(clientKey string) bool {
	f.asked = append(f.asked, clientKey)
	return f.present[clientKey]
}

func testCluster(name string) *v3.Cluster {
	return &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// TestHasSessionProbesTheClusterAgentSession is the substance of the fix. Connected used to be
// computed from the steve aggregation session, which is a different session, opened later, for the
// Dashboard's benefit. Every consumer of Connected reaches the cluster through the agent's own
// tunnel session instead — the one pkg/dialer.Factory.clusterDialer looks up by bare cluster name.
func TestHasSessionProbesTheClusterAgentSession(t *testing.T) {
	cluster := testCluster("c-m-abcde")

	t.Run("true when the cluster agent session exists", func(t *testing.T) {
		sessions := &fakeSessions{present: map[string]bool{cluster.Name: true}}
		c := &checker{tunnelServer: sessions}

		assert.True(t, c.hasSession(cluster))
		assert.Equal(t, []string{cluster.Name}, sessions.asked,
			"must probe the cluster's own tunnel session, keyed on the bare cluster name")
	})

	t.Run("false when only the steve aggregation session exists", func(t *testing.T) {
		// The regression this guards: an agent whose steve aggregation has not come up, or does
		// not come up at all, is still perfectly reachable and must count as connected. The
		// inverse — only steve present — must not.
		sessions := &fakeSessions{present: map[string]bool{proxy.Prefix + cluster.Name: true}}
		c := &checker{tunnelServer: sessions}

		assert.False(t, c.hasSession(cluster))
		assert.NotContains(t, sessions.asked, proxy.Prefix+cluster.Name,
			"the steve aggregation session is not what Connected means")
	})

	t.Run("false when there is no session", func(t *testing.T) {
		sessions := &fakeSessions{present: map[string]bool{}}
		c := &checker{tunnelServer: sessions}

		assert.False(t, c.hasSession(cluster))
	})
}

// TestHasSessionIgnoresClusterProvenance pins the property that motivated this change: the check
// must not depend on how the cluster was created. All of these clusters are treated identically,
// because they all establish the agent tunnel in the same way.
func TestHasSessionIgnoresClusterProvenance(t *testing.T) {
	tests := []struct {
		name    string
		cluster *v3.Cluster
	}{
		{
			name:    "imported",
			cluster: &v3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-abcde"}},
		},
		{
			name: "node driver v2prov",
			cluster: &v3.Cluster{ObjectMeta: metav1.ObjectMeta{
				Name:        "c-m-abcde",
				Annotations: map[string]string{"provisioning.cattle.io/administrated": "true"},
			}},
		},
		{
			name: "hosted",
			cluster: &v3.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "c-hosted"},
				Status:     v3.ClusterStatus{Driver: "EKS"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := &fakeSessions{present: map[string]bool{tt.cluster.Name: true}}
			c := &checker{tunnelServer: sessions}

			assert.True(t, c.hasSession(tt.cluster))
			assert.Equal(t, []string{tt.cluster.Name}, sessions.asked)
		})
	}
}

// TestIsClusterSessionKey covers the tunnel hook's filter. A connecting agent authorizes several
// keys and only the cluster's own session is the one hasSession goes on to probe, so promoting on
// any of the others would either be redundant or plain wrong.
func TestIsClusterSessionKey(t *testing.T) {
	tests := []struct {
		clientKey string
		want      bool
	}{
		{clientKey: "c-m-abcde", want: true},
		{clientKey: "local", want: true},
		{clientKey: "c-abcde", want: true},

		// Steve aggregation session.
		{clientKey: proxy.Prefix + "c-m-abcde", want: false},
		// Per-node session.
		{clientKey: "c-m-abcde:node-1", want: false},
		// A steve key that also has a node suffix, for completeness.
		{clientKey: proxy.Prefix + "c-m-abcde:node-1", want: false},

		{clientKey: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.clientKey, func(t *testing.T) {
			assert.Equal(t, tt.want, isClusterSessionKey(tt.clientKey))
		})
	}
}

// TestCheckClusterDoesNotSuppressPreBootstrap covers the deadlock this condition used to cause.
//
// checkCluster used to force Connected false for any pre-bootstrapping cluster, to stop
// provisioning advancing. But PreBootstrapped is set by managementuser/secret, a downstream
// controller, downstream controllers are started by usercontrollers, and usercontrollers waits on
// Connected — so PreBootstrapped could never be set and the cluster hung until the test timed out.
//
// Connected must now simply report the tunnel. The pre-bootstrap rule lives on
// RKEControlPlane.Status.AgentConnected instead.
func TestCheckClusterDoesNotSuppressPreBootstrap(t *testing.T) {
	features.ProvisioningPreBootstrap.Set(true)
	t.Cleanup(func() { features.ProvisioningPreBootstrap.Set(false) })

	// A v2prov cluster mid-pre-bootstrap: administrated, and with no PreBootstrapped condition,
	// which is exactly the state capr.PreBootstrap reports true for.
	cluster := &v3.Cluster{ObjectMeta: metav1.ObjectMeta{
		Name:        "c-m-vksz879q",
		Annotations: map[string]string{"provisioning.cattle.io/administrated": "true"},
	}}
	require.True(t, capr.PreBootstrap(cluster), "test setup: cluster should be pre-bootstrapping")

	ctrl := gomock.NewController(t)
	clusters := fake.NewMockNonNamespacedControllerInterface[*v3.Cluster, *v3.ClusterList](ctrl)
	clusters.EXPECT().Get(cluster.Name, gomock.Any()).Return(cluster.DeepCopy(), nil)

	var got *v3.Cluster
	clusters.EXPECT().UpdateStatus(gomock.Any()).DoAndReturn(func(c *v3.Cluster) (*v3.Cluster, error) {
		got = c
		return c, nil
	})

	c := &checker{
		clusters:     clusters,
		tunnelServer: &fakeSessions{present: map[string]bool{cluster.Name: true}},
	}

	require.NoError(t, c.checkCluster(cluster))
	require.NotNil(t, got, "checkCluster must write the condition")
	assert.True(t, Connected.IsTrue(got),
		"a pre-bootstrapping cluster whose agent is connected must be reported as connected")
}
