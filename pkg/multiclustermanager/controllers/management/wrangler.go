package management

import (
	"context"

	"github.com/rancher/rancher/pkg/multiclustermanager/clustermanager"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/eks"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/eksupstreamrefresh"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/systemcharts"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) error {
	k3sbasedupgrade.Register(ctx, wranglerContext, management, manager)
	eks.Register(ctx, wranglerContext, management)
	eksupstreamrefresh.Register(ctx, wranglerContext)
	return systemcharts.Register(ctx, wranglerContext)
}
