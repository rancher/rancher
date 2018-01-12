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
	machineController.Register(management)
	catalogController.Register(ctx, management)
	clusterheartbeat.Register(ctx, management)
	clusterprovisioner.Register(management)
	clusterstats.Register(management)
	agent.Register(ctx, management)
	clusterevents.Register(ctx, management)
	auth.Register(ctx, management)
}
