package steve

import (
	"context"

	"github.com/rancher/rancher/pkg/api/steve/catalog"
	"github.com/rancher/rancher/pkg/api/steve/clusters"
	"github.com/rancher/rancher/pkg/api/steve/userpreferences"
	"github.com/rancher/rancher/pkg/wrangler"
	steve "github.com/rancher/steve/pkg/server"
)

func Setup(ctx context.Context, server *steve.Server, config *wrangler.Context) error {
	userpreferences.Register(server.BaseSchemas, server.ClientFactory)
	if err := clusters.Register(ctx, server); err != nil {
		return err
	}
	return catalog.Register(ctx,
		server,
		config.HelmOperations,
		config.CatalogContentManager)
}
