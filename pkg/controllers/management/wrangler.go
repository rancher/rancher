package management

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/management/agentauth"
	"github.com/rancher/rancher/pkg/wrangler/generated/controllers/management.cattle.io"
	"github.com/rancher/types/config"
	"github.com/rancher/wrangler-api/pkg/generated/controllers/rbac"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/start"
	"github.com/sirupsen/logrus"
)

func wrangler(ctx context.Context, managementContext *config.ManagementContext) {
	if err := wranglerErr(ctx, managementContext); err != nil {
		logrus.Fatal("Failed to start wrangler management context", err)
	}
}

func wranglerErr(ctx context.Context, managementContext *config.ManagementContext) error {
	rbac := rbac.NewFactoryFromConfigOrDie(&managementContext.RESTConfig)
	apply, err := apply.NewForConfig(&managementContext.RESTConfig)
	if err != nil {
		return err
	}
	mgmt := management.NewFactoryFromConfigOrDie(&managementContext.RESTConfig)

	if err := agentauth.Register(ctx, mgmt.Management().V3().Cluster(), rbac.Rbac().V1(), apply); err != nil {
		return err
	}

	return start.All(ctx, 5, rbac, mgmt)
}
