package clusterconnected

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/proxy"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	managementcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Connected reports one fact and one fact only: the cluster's agent currently has a live tunnel
// session, so Rancher can reach the cluster. It says nothing about how the cluster was created,
// whether its API is healthy, or whether any particular lifecycle step is allowed to proceed.
//
// Keep it that way. This condition has twice been overloaded with something else and both times
// it deadlocked provisioning:
//
//   - It was computed from the steve aggregation session, which only exists to serve the
//     Dashboard and comes up after the agent tunnel. See hasSession.
//   - It was forced false while a cluster was pre-bootstrapping, to hold back provisioning. But
//     PreBootstrapped is set by a downstream controller (managementuser/secret), downstream
//     controllers are started by usercontrollers, and usercontrollers waits on Connected — so
//     nothing could ever set PreBootstrapped. That gating now lives on
//     RKEControlPlane.Status.AgentConnected, which is the thing provisioning actually reads.
//
// If you need "connected, and also X", write "Connected.IsTrue(c) && X" at the consumer that
// needs X, rather than folding X in here for every consumer.
var (
	Connected = condition.Cond("Connected")
)

const (
	// checkInterval is how often every cluster's connectivity is polled. This is the only
	// mechanism that can clear the Connected condition, so it also bounds how long a
	// disconnect goes unnoticed.
	checkInterval = 15 * time.Second

	// sessionSettleDelay is how long the tunnel hook waits before probing a cluster. The
	// authorizer callback fires before the session is registered with the tunnel server,
	// so probing immediately would always miss.
	sessionSettleDelay = 2 * time.Second

	// hookRateLimit is the minimum gap between two hook-driven checks of the same cluster.
	hookRateLimit = 5 * time.Second

	// hookQueueSize bounds the pending hook-driven checks. Overflow is dropped rather than
	// blocking the tunnel request; the ticker is the backstop.
	hookQueueSize = 100
)

func Register(ctx context.Context, wrangler *wrangler.Context) {
	c := newChecker(wrangler)

	go func() {
		for range ticker.Context(ctx, checkInterval) {
			if err := c.check(); err != nil {
				logrus.Errorf("failed to check cluster connectivity: %v", err)
			}
		}
	}()
}

// RegisterTunnelHook promotes a cluster's Connected condition to true as soon as its agent
// authorizes a tunnel session, instead of waiting up to checkInterval for the next poll.
// Provisioning is gated on this condition (via RKEControlPlane.Status.AgentConnected), so
// the poll interval is otherwise paid in full on every new cluster.
//
// Unlike Register this must run on every Rancher pod, not just the leader: a tunnel connect
// lands on whichever pod the load balancer picked, and only that pod sees the callback.
//
// The hook is deliberately promote-only. It never sets Connected to false, so a spurious or
// duplicated callback cannot report a false disconnect; clearing the condition remains the
// ticker's job.
func RegisterTunnelHook(ctx context.Context, wrangler *wrangler.Context) {
	if wrangler.TunnelAuthorizer == nil {
		return
	}

	c := newChecker(wrangler)
	queue := make(chan string, hookQueueSize)

	wrangler.TunnelAuthorizer.OnAuthorized(func(clientKey string) {
		// Several kinds of key are authorized for a connecting agent: the cluster's own tunnel
		// session, keyed on the bare cluster name; per-node sessions, keyed
		// "<cluster>:<node>"; and the steve aggregation session, keyed on
		// proxy.Prefix + cluster name. Only the first is what hasSession probes.
		if !isClusterSessionKey(clientKey) {
			return
		}
		select {
		case queue <- clientKey:
		default:
			logrus.Debugf("[clusterConnectedCondition] dropping tunnel hook for %s, queue is full", clientKey)
		}
	})

	go c.processTunnelHooks(ctx, queue)
}

// isClusterSessionKey reports whether a tunnel client key is a cluster's own agent session, as
// opposed to a per-node session ("<cluster>:<node>") or the steve aggregation session
// (proxy.Prefix + cluster name). A cluster session key is just the cluster name.
func isClusterSessionKey(clientKey string) bool {
	return clientKey != "" &&
		!strings.HasPrefix(clientKey, proxy.Prefix) &&
		!strings.Contains(clientKey, ":")
}

func newChecker(wrangler *wrangler.Context) *checker {
	return &checker{
		clusterCache: wrangler.Mgmt.Cluster().Cache(),
		clusters:     wrangler.Mgmt.Cluster(),
		tunnelServer: wrangler.TunnelServer,
	}
}

func (c *checker) processTunnelHooks(ctx context.Context, queue <-chan string) {
	lastCheck := map[string]time.Time{}

	for {
		select {
		case <-ctx.Done():
			return
		case clusterName := <-queue:
			if time.Since(lastCheck[clusterName]) < hookRateLimit {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(sessionSettleDelay):
			}

			lastCheck[clusterName] = time.Now()
			if err := c.promoteCluster(clusterName); err != nil {
				logrus.Debugf("[clusterConnectedCondition] failed to promote cluster %s on tunnel connect: %v", clusterName, err)
			}
		}
	}
}

// promoteCluster sets Connected to true if the cluster now has a working tunnel session. It
// is a no-op in every other case, including when the session has already gone away again.
func (c *checker) promoteCluster(clusterName string) error {
	cluster, err := c.clusterCache.Get(clusterName)
	if err != nil {
		return err
	}

	if Connected.IsTrue(cluster) || !c.hasSession(cluster) {
		return nil
	}

	logrus.Debugf("[clusterConnectedCondition] promoting cluster %v to connected on tunnel connect", clusterName)
	return c.updateClusterConnectedCondition(cluster, true)
}

// sessionChecker is the part of the remotedialer server this controller needs. Narrowed to an
// interface so that tests can assert which session key is probed, which is the whole substance of
// what Connected means.
type sessionChecker interface {
	HasSession(clientKey string) bool
}

type checker struct {
	clusterCache managementcontrollers.ClusterCache
	clusters     managementcontrollers.ClusterClient
	tunnelServer sessionChecker
}

func (c *checker) check() error {
	clusters, err := c.clusterCache.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		if err := c.checkCluster(cluster); err != nil {
			logrus.Errorf("failed to check connectivity of cluster [%s]: %v", cluster.Name, err)
		}
	}
	return nil
}

// hasSession reports whether the cluster's agent has a live tunnel session.
//
// That is deliberately all this asks, because it is what every consumer of Connected actually
// depends on. clusterdeploy and usercontrollers both go straight on to build a downstream client;
// managesystemagent writes downstream resources; rkecontrolplane copies this into
// Status.AgentConnected. All of them ride pkg/dialer.Factory.clusterDialer, which dials through
// exactly this session — "if f.TunnelServer.HasSession(cluster.Name)" in factory.go. So the
// cluster's own tunnel session, keyed on the bare cluster name, is the honest predicate.
//
// Two things this intentionally does not do:
//
// It does not probe the steve aggregation session (proxy.Prefix + cluster name), which is what it
// used to do. That is a different session for a different purpose: the agent only writes the
// stv-aggregation secret from onConnect, so it comes up strictly after the tunnel above, and it
// exists to let the Dashboard proxy into the cluster. Nothing consuming Connected needs it, and
// the steve proxy checks HasSession for itself in pkg/api/steve/aggregation. Probing it made
// Connected mean "the Dashboard can browse this cluster", and gated cluster lifecycle work on a
// component that has nothing to do with whether Rancher can talk to the cluster.
//
// It also does not check that the downstream API answers. That is ClusterConditionReady's job,
// which healthsyncer owns and which reports far more detail than a bool. Folding reachability in
// here would make one condition mean two things and would report clusters whose API dialing does
// not go through the tunnel at all — public cloud drivers take a direct dialer in factory.go — as
// disconnected. A dead session is still noticed: remotedialer pings every PingWriteInterval and
// drops the session when the read deadline passes, and the server removes it on close.
func (c *checker) hasSession(cluster *v3.Cluster) bool {
	return c.tunnelServer.HasSession(cluster.Name)
}

func (c *checker) checkCluster(cluster *v3.Cluster) error {
	if cluster.Spec.Internal {
		if !Connected.IsTrue(cluster) {
			return c.updateClusterConnectedCondition(cluster, true)
		}
		return nil
	}

	hasSession := c.hasSession(cluster)
	// The simpler condition of hasSession == Connected.IsTrue(cluster) is not
	// used because it treats a non-existent conditions as False
	if hasSession && Connected.IsTrue(cluster) {
		return nil
	} else if !hasSession && Connected.IsFalse(cluster) && v3.ClusterConditionReady.GetReason(cluster) == "Disconnected" {
		return nil
	}

	return c.updateClusterConnectedCondition(cluster, hasSession)
}

func (c *checker) updateClusterConnectedCondition(cluster *v3.Cluster, connected bool) error {
	if cluster == nil {
		return fmt.Errorf("cluster cannot be nil")
	}
	for i := 0; i < 3; i++ {
		latestCluster, err := c.clusters.Get(cluster.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		Connected.SetStatusBool(latestCluster, connected)
		if !connected && v3.ClusterConditionProvisioned.IsTrue(latestCluster) {
			// For v2prov clusters, only update Ready when the condition is not being managed by the provisioner.
			if !latestCluster.Status.ReadyReconciling {
				v3.ClusterConditionReady.False(latestCluster)
				v3.ClusterConditionReady.Reason(latestCluster, "Disconnected")
				v3.ClusterConditionReady.Message(latestCluster, "Cluster agent is not connected")
			}
		}
		logrus.Tracef("[clusterConnectedCondition] update cluster %v", cluster.Name)
		_, err = c.clusters.UpdateStatus(latestCluster)
		if apierror.IsConflict(err) {
			continue
		}
		return err
	}
	return fmt.Errorf("unable to update cluster connected condition")
}
