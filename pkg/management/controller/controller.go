package controller

import (
	"context"

	catalogController "github.com/rancher/rancher/pkg/catalog/controller"
	"github.com/rancher/rancher/pkg/dialer"
	machineController "github.com/rancher/rancher/pkg/machine/controller"
	"github.com/rancher/rancher/pkg/management/controller/agent"
	"github.com/rancher/rancher/pkg/management/controller/auth"
	"github.com/rancher/rancher/pkg/management/controller/clusterevents"
	"github.com/rancher/rancher/pkg/management/controller/clusterprovisioner"
	"github.com/rancher/rancher/pkg/management/controller/clusterstats"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext, dialerFactory dialer.Factory) {
	// auth handlers need to run early to create namespaces that back clusters and projects
	// also, these handlers are purely in the mgmt plane, so they are lightweight compared to those that interact with machines and clusters
	auth.RegisterEarly(ctx, management)
	agent.Register(ctx, management)
	machineController.Register(management)
	catalogController.Register(ctx, management)
	clusterstats.Register(management)
	clusterevents.Register(ctx, management)
	clusterprovisioner.Register(management, dialerFactory)
	auth.RegisterLate(ctx, management)
	registerClusterScopedGC(ctx, management)
}
