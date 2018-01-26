package controller

import (
	"context"

	catalogController "github.com/rancher/catalog-controller/controller"
	"github.com/rancher/cluster-controller/controller/agent"
	"github.com/rancher/cluster-controller/controller/auth"
	"github.com/rancher/cluster-controller/controller/clusterevents"
	"github.com/rancher/cluster-controller/controller/clusterheartbeat"
	"github.com/rancher/cluster-controller/controller/clusterprovisioner"
	"github.com/rancher/cluster-controller/controller/clusterstats"
	machineController "github.com/rancher/machine-controller/controller"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	// auth handlers need to run early to create namespaces that back clusters and projects
	// also, these handlers are purely in the mgmt plane, so they are lightweight compared to those that interact with machines and clusters
	auth.RegisterEarly(ctx, management)
	agent.Register(ctx, management)
	machineController.Register(management)
	catalogController.Register(ctx, management)
	clusterheartbeat.Register(ctx, management)
	clusterstats.Register(management)
	clusterevents.Register(ctx, management)
	clusterprovisioner.Register(management)
	auth.RegisterLate(ctx, management)
	registerClusterScopedGC(ctx, management)
}
