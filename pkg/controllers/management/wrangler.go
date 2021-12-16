package management

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/aks"
	"github.com/rancher/rancher/pkg/controllers/management/authprovisioningv2"
	"github.com/rancher/rancher/pkg/controllers/management/clusterconnected"
	"github.com/rancher/rancher/pkg/controllers/management/clusterupstreamrefresher"
	"github.com/rancher/rancher/pkg/controllers/management/eks"
	"github.com/rancher/rancher/pkg/controllers/management/feature"
	"github.com/rancher/rancher/pkg/controllers/management/gke"
	"github.com/rancher/rancher/pkg/controllers/management/k3sbasedupgrade"
	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
)

func RegisterWrangler(ctx context.Context, wranglerContext *wrangler.Context, management *config.ManagementContext, manager *clustermanager.Manager) error {
	k3sbasedupgrade.Register(ctx, wranglerContext, management, manager)
	aks.Register(ctx, wranglerContext, management)
	eks.Register(ctx, wranglerContext, management)
	gke.Register(ctx, wranglerContext, management)
	clusterupstreamrefresher.Register(ctx, wranglerContext)

	feature.Register(ctx, wranglerContext)
	clusterconnected.Register(ctx, wranglerContext)

	if features.ProvisioningV2.Enabled() {
		if err := authprovisioningv2.Register(ctx, wranglerContext, management); err != nil {
			return err
		}
	}

	return nil
}
