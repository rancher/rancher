package clusterconnected

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/proxy"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/capr"
	managementcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/remotedialer"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	"github.com/sirupsen/logrus"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	Connected = condition.Cond("Connected")
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
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	return resp.StatusCode == http.StatusOK
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

	// RKE2: wait to update the connected condition until it is pre-bootstrapped
	if capr.PreBootstrap(cluster) &&
		cluster.Annotations["provisioning.cattle.io/administrated"] == "true" &&
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
	for range 3 {
		cluster = cluster.DeepCopy()
		Connected.SetStatusBool(cluster, connected)
		if !connected && v3.ClusterConditionProvisioned.IsTrue(cluster) {
			v3.ClusterConditionReady.False(cluster)
			v3.ClusterConditionReady.Reason(cluster, "Disconnected")
			v3.ClusterConditionReady.Message(cluster, "Cluster agent is not connected")
		}
		logrus.Tracef("[clusterConnectedCondition] update cluster %v", cluster.Name)
		_, err := c.clusters.Update(cluster)
		if apierror.IsConflict(err) {
			cluster, err = c.clusters.Get(cluster.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			continue
		}
		return err
	}
	return fmt.Errorf("unable to update cluster connected condition")
}
