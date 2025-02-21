package k3sbasedupgrade

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/clustermanager"
	wranglerv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/systemaccount"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

type handler struct {
	systemUpgradeNamespace string
	clusterCache           wranglerv3.ClusterCache
	clusterClient          wranglerv3.ClusterClient
	nodeLister             v3.NodeLister
	systemAccountManager   *systemaccount.Manager
	manager                *clustermanager.Manager
	clusterEnqueueAfter    func(name string, duration time.Duration)
	ctx                    context.Context
}

const (
	systemUpgradeNS        = "cattle-system"
	upgradeDisableLabelKey = "upgrade.cattle.io/disable"

	RancherManagedPlan = "rancher-managed"
)

func Register(ctx context.Context, wContext *wrangler.Context, mgmtCtx *config.ManagementContext, manager *clustermanager.Manager) {
	h := &handler{
		systemUpgradeNamespace: systemUpgradeNS,
		clusterCache:           wContext.Mgmt.Cluster().Cache(),
		clusterClient:          wContext.Mgmt.Cluster(),
		clusterEnqueueAfter:    wContext.Mgmt.Cluster().EnqueueAfter,
		nodeLister:             mgmtCtx.Management.Nodes("").Controller().Lister(),
		systemAccountManager:   systemaccount.NewManager(mgmtCtx),
		manager:                manager,
		ctx:                    ctx,
	}
	wContext.Mgmt.Cluster().OnChange(ctx, "k3s-upgrade-controller", h.onClusterChange)
}
