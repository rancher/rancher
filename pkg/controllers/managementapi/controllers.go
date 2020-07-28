package managementapi

import (
	"context"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementapi/catalog"
	"github.com/rancher/rancher/pkg/controllers/managementapi/dynamicschema"
	"github.com/rancher/rancher/pkg/controllers/managementapi/k3smetadata"
	"github.com/rancher/rancher/pkg/controllers/managementapi/samlconfig"
	"github.com/rancher/rancher/pkg/controllers/managementapi/usercontrollers"
	whitelistproxyKontainerDriver "github.com/rancher/rancher/pkg/controllers/managementapi/whitelistproxy/kontainerdriver"
	whitelistproxyNodeDriver "github.com/rancher/rancher/pkg/controllers/managementapi/whitelistproxy/nodedriver"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager, server *normanapi.Server) error {
	catalog.Register(ctx, scaledContext)
	dynamicschema.Register(ctx, scaledContext, server.Schemas)
	whitelistproxyNodeDriver.Register(ctx, scaledContext)
	whitelistproxyKontainerDriver.Register(ctx, scaledContext)
	samlconfig.Register(ctx, scaledContext)
	k3smetadata.Register(ctx, scaledContext)
	usercontrollers.Register(ctx, scaledContext, clusterManager)
	return nil
}
