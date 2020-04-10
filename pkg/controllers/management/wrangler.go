package management

import (
	"context"
	"github.com/rancher/rancher/pkg/controllers/management/clusterapi"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/k3supgrade"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/types/config"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	// Add controllers to register here

	k3supgrade.Register(ctx, wranglerContext, management, manager)
	clusterapi.Register(ctx, wranglerContext, management, manager)

}
