package steve

import (
	"context"

	"github.com/rancher/rancher/pkg/api/steve/catalog"
	"github.com/rancher/rancher/pkg/wrangler"
	steve "github.com/rancher/steve/pkg/server"
)

func Setup(ctx context.Context, server *steve.Server, config *wrangler.Context) error {
	return catalog.Register(ctx,
		server,
		config.Core.Secret(),
		config.Core.Pod(),
		config.Core.ConfigMap(),
		config.Catalog)
}
