package helm

import (
	"context"

	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) {
	RegisterRepos(ctx,
		wrangler.Core.Secret(),
		wrangler.Catalog.Repo(),
		wrangler.Catalog.ClusterRepo(),
		wrangler.Core.ConfigMap())
	RegisterReleases(ctx,
		wrangler.Apply,
		wrangler.ControllerFactory.SharedCacheFactory().SharedClientFactory(),
		wrangler.Core.ConfigMap(),
		wrangler.Core.Secret(),
		wrangler.Catalog.Release())
	RegisterOperations(ctx,
		wrangler.Core.Pod(),
		wrangler.Catalog.Operation())
}
