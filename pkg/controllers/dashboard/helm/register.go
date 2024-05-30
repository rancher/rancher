package helm

import (
	"context"

	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) {
	RegisterRepos(ctx,
		wrangler.Apply,
		wrangler.Core.Secret().Cache(),
		wrangler.Catalog.ClusterRepo(),
		wrangler.Core.ConfigMap(),
		wrangler.Core.ConfigMap().Cache())
	RegisterOCIRepo(ctx,
		wrangler.Apply,
		wrangler.Catalog.ClusterRepo(),
		wrangler.Core.ConfigMap(),
		wrangler.Core.Secret().Cache())
	RegisterApps(ctx,
		wrangler.Apply,
		wrangler.ControllerFactory.SharedCacheFactory().SharedClientFactory(),
		wrangler.Core.ConfigMap(),
		wrangler.Core.Secret(),
		wrangler.Catalog.App())
	RegisterOperations(ctx,
		wrangler.K8s,
		wrangler.Core.Pod(),
		wrangler.Catalog.Operation())
}
