package dashboard

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/controllers/dashboard/kubernetesprovider"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	helm.Register(ctx, wrangler)
	kubernetesprovider.Register(ctx,
		wrangler.Mgmt.Cluster(),
		wrangler.K8s,
		wrangler.MultiClusterManager)
	return nil
}
