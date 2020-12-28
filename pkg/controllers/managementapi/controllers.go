package managementapi

import (
	"context"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	podsecuritypolicy2 "github.com/rancher/rancher/pkg/controllers/management/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/managementagent/monitoring"
	"github.com/rancher/rancher/pkg/controllers/managementapi/catalog"
	"github.com/rancher/rancher/pkg/controllers/managementapi/dynamicschema"
	"github.com/rancher/rancher/pkg/controllers/managementapi/k3smetadata"
	"github.com/rancher/rancher/pkg/controllers/managementapi/samlconfig"
	"github.com/rancher/rancher/pkg/controllers/managementapi/usercontrollers"
	whitelistproxyKontainerDriver "github.com/rancher/rancher/pkg/controllers/managementapi/whitelistproxy/kontainerdriver"
	whitelistproxyNodeDriver "github.com/rancher/rancher/pkg/controllers/managementapi/whitelistproxy/nodedriver"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager, server *normanapi.Server) error {
	if err := registerIndexers(ctx, scaledContext); err != nil {
		return err
	}

	catalog.Register(ctx, scaledContext)
	dynamicschema.Register(ctx, scaledContext, server.Schemas)
	whitelistproxyNodeDriver.Register(ctx, scaledContext)
	whitelistproxyKontainerDriver.Register(ctx, scaledContext)
	samlconfig.Register(ctx, scaledContext)
	k3smetadata.Register(ctx, scaledContext)
	usercontrollers.Register(ctx, scaledContext, clusterManager)
	return nil
}

func registerIndexers(ctx context.Context, scaledContext *config.ScaledContext) error {
	if err := clusterauthtoken.RegisterIndexers(ctx, scaledContext); err != nil {
		return err
	}
	if err := rbac.RegisterIndexers(ctx, scaledContext); err != nil {
		return err
	}
	if err := monitoring.RegisterIndexers(ctx, scaledContext); err != nil {
		return err
	}
	if err := auth.RegisterIndexers(ctx, scaledContext); err != nil {
		return err
	}
	if err := tokens.RegisterIndexer(ctx, scaledContext); err != nil {
		return err
	}
	if err := podsecuritypolicy.RegisterIndexers(ctx, scaledContext); err != nil {
		return err
	}
	if err := podsecuritypolicy2.RegisterIndexers(ctx, scaledContext); err != nil {
		return err
	}
	return nil
}
