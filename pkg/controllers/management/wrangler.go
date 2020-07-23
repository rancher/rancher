package management

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) error {
	// Add controllers to register here

	k3sbasedupgrade.Register(ctx, wranglerContext, management, manager)

	if err := dashboard.Register(ctx, wranglerContext); err != nil {
		return err
	}

	return nil
}
