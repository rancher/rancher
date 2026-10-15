package clusterconnected

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/proxy"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	pkgcluster "github.com/rancher/rancher/pkg/cluster"
	managementcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func Register(ctx context.Context, wrangler *wrangler.Context) {
	c := checker{
		clusterCache: wrangler.Mgmt.Cluster().Cache(),
		clusters:     wrangler.Mgmt.Cluster(),
		tunnelServer: wrangler.TunnelServer,
	}

	go func() {
		for range ticker.Context(ctx, 15*time.Second) {
			if err := c.check(); err != nil {
				logrus.Errorf("failed to check cluster connectivity: %v", err)
			}
		}
	}()
}

type checker struct {
	clusterCache managementcontrollers.ClusterCache
	clusters     managementcontrollers.ClusterClient
	tunnelServer *remotedialer.Server
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

func (c *checker) hasSession(cluster *v3.Cluster) bool {
	clientKey := proxy.Prefix + cluster.Name
	hasSession := c.tunnelServer.HasSession(clientKey)
	if !hasSession {
		return false
	}

	dialer := c.tunnelServer.Dialer(clientKey)
	transport := &http.Transport{
		DialContext: dialer,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
	}
	resp, err := client.Get("http://not-used/ping")
	if err != nil {
		return false
	}
	defer func() {
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}()
	return resp.StatusCode == http.StatusOK
}

func (c *checker) checkCluster(cluster *v3.Cluster) error {
	if cluster.Spec.Internal {
		if !v3.ClusterConditionConnected.IsTrue(cluster) {
			return c.updateClusterConnectedCondition(cluster, true)
		}
		return nil
	}

	hasSession := c.hasSession(cluster)
	// The simpler condition of hasSession == Connected.IsTrue(cluster) is not
	// used because it treats a non-existent conditions as False
	if hasSession && v3.ClusterConditionConnected.IsTrue(cluster) {
		return nil
	} else if !hasSession && v3.ClusterConditionConnected.IsFalse(cluster) {
		return nil
	}

	// RKE2: wait to update the connected condition until it is pre-bootstrapped
	if capr.ShouldPreBootstrap(cluster) && pkgcluster.IsAdministrated(cluster) &&
		cluster.Name != "local" {
		// overriding it to be disconnected until bootstrapping is done
		logrus.Debugf("[pre-bootstrap][%v] Waiting for cluster to be pre-bootstrapped - not marking agent connected", cluster.Name)
		return c.updateClusterConnectedCondition(cluster, false)
	}

	return c.updateClusterConnectedCondition(cluster, hasSession)
}

func (c *checker) updateClusterConnectedCondition(cluster *v3.Cluster, connected bool) error {
	if cluster == nil {
		return fmt.Errorf("cluster cannot be nil")
	}

	clusterName := cluster.Name
	for range 3 {
		cluster, err := c.clusters.Get(clusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		v3.ClusterConditionConnected.SetStatusBool(cluster, connected)
		pkgcluster.SummarizeCondition(cluster)

		logrus.Tracef("[clusterConnectedCondition] update cluster %v", cluster.Name)

		_, err = c.clusters.UpdateStatus(cluster)
		if apierror.IsConflict(err) {
			continue
		}
		return err
	}
	return fmt.Errorf("unable to update cluster connected condition")
}
