package clusters

import (
	rbacapi "github.com/rancher/rancher/tests/e2e/actions/kubeapi/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/wrangler"
)

// GetClusterWranglerContext returns the context for the cluster
func GetClusterWranglerContext(client *rancher.Client, clusterID string) (*wrangler.Context, error) {
	if clusterID == rbacapi.LocalCluster {
		return client.WranglerContext, nil
	}

	return client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
}
