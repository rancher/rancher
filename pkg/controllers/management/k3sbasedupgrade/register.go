package k3sbasedupgrade

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/clustermanager"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

type handler struct {
	ctx                 context.Context
	clusterClient       wranglerv3.ClusterClient
	nodeLister          v3.NodeLister
	manager             *clustermanager.Manager
	clusterEnqueueAfter func(name string, duration time.Duration)
}

const (
	systemUpgradeNS        = "cattle-system"
	rancherManagedPlan     = "rancher-managed"
	upgradeDisableLabelKey = "upgrade.cattle.io/disable"
)

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext, manager *clustermanager.Manager) {
	h := &handler{
		ctx:                 ctx,
		clusterClient:       wContext.Mgmt.Cluster(),
		nodeLister:          mgmtCtx.Management.Nodes("").Controller().Lister(),
		manager:             manager,
		clusterEnqueueAfter: wContext.Mgmt.Cluster().EnqueueAfter,
	}
	wContext.Mgmt.Cluster().OnChange(ctx, "k3s-upgrade-controller", h.onClusterChange)
}
