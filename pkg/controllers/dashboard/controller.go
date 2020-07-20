package dashboard

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	helm.RegisterRepos(ctx,
		wrangler.Core.Secret(),
		wrangler.Catalog.Repo(),
		wrangler.Catalog.ClusterRepo(),
		wrangler.Core.ConfigMap())
	helm.RegisterReleases(ctx,
		wrangler.Apply,
		wrangler.ControllerFactory.SharedCacheFactory().SharedClientFactory(),
		wrangler.Core.ConfigMap(),
		wrangler.Core.Secret(),
		wrangler.Catalog.Release())
	return nil
}
