package management

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/types/config"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	// Add controllers to register here
}
