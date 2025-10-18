package windows

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, cluster *v3.Cluster, userContext *config.UserContext) {
	if !cluster.Spec.WindowsPreferedCluster {
		return
	}
	node := &NodeTaintsController{
		nodeClient: userContext.Corew.Node(),
	}
	userContext.Corew.Node().OnChange(ctx, "linux-node-taints-handler", node.sync)
}
