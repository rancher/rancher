package managementapi

import (
	"context"

	normanapi "github.com/rancher/norman/api"
	"github.com/rancher/rancher/pkg/auth/tokens"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	v3cluster "github.com/rancher/rancher/pkg/controllers/management/cluster"
	"github.com/rancher/rancher/pkg/features"

	"github.com/rancher/rancher/pkg/controllers/managementapi/dynamicschema"
	"github.com/rancher/rancher/pkg/controllers/managementapi/samlconfig"
	"github.com/rancher/rancher/pkg/controllers/managementapi/usercontrollers"
	whitelistproxyKontainerDriver "github.com/rancher/rancher/pkg/controllers/managementapi/whitelistproxy/kontainerdriver"
	whitelistproxyNodeDriver "github.com/rancher/rancher/pkg/controllers/managementapi/whitelistproxy/nodedriver"
	"github.com/rancher/rancher/pkg/controllers/managementuser/clusterauthtoken"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac/roletemplates"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager, server *normanapi.Server) error {
	if err := registerIndexers(scaledContext); err != nil {
		return err
	}

	dynamicschema.Register(ctx, scaledContext, server.Schemas)
	whitelistproxyNodeDriver.Register(ctx, scaledContext)
	whitelistproxyKontainerDriver.Register(ctx, scaledContext)
	samlconfig.Register(ctx, scaledContext)
	usercontrollers.Register(ctx, scaledContext, clusterManager)
	return nil
}

func registerIndexers(scaledContext *config.ScaledContext) error {
	if err := clusterauthtoken.RegisterIndexers(scaledContext); err != nil {
		return err
	}
	if err := rbac.RegisterIndexers(scaledContext); err != nil {
		return err
	}
	if err := auth.RegisterIndexers(scaledContext); err != nil {
		return err
	}
	if err := tokens.RegisterIndexer(scaledContext); err != nil {
		return err
	}

	// Using aggregated cluster roles is currently experimental and only available via feature flags.
	if features.AggregatedRoleTemplates.Enabled() {
		roletemplates.RegisterIndexers(scaledContext.Wrangler)
	}

	v3cluster.RegisterIndexers(scaledContext)
	return nil
}
