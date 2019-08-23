package windows

import (
	"context"

	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, cluster *v3.Cluster, userContext *config.UserContext) {
	if !cluster.Spec.WindowsPreferedCluster {
		return
	}
	clusterName := userContext.ClusterName
	node := &NodeTaintsController{
		nodeClient: userContext.Management.Management.Nodes(clusterName),
	}
	userContext.Management.Management.Nodes(clusterName).AddClusterScopedHandler(ctx, "linux-node-taints-handler", clusterName, node.sync)
}
