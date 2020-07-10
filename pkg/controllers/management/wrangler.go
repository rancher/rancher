package management

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/k3supgrade"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	// Add controllers to register here

	k3supgrade.Register(ctx, wranglerContext, management, manager)

}
