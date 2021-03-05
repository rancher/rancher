package management

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/clusterupstreamrefresher"
	"github.com/rancher/rancher/pkg/controllers/management/eks"
	"github.com/rancher/rancher/pkg/controllers/management/gke"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/controllers/management/systemcharts"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) error {
	k3sbasedupgrade.Register(ctx, wranglerContext, management, manager)
	eks.Register(ctx, wranglerContext, management)
	gke.Register(ctx, wranglerContext, management)
	clusterupstreamrefresher.Register(ctx, wranglerContext)
	return systemcharts.Register(ctx, wranglerContext)
}
