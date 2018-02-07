package management

import (
	"context"

	"github.com/rancher/rancher/pkg/controllers/management/agent"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/controllers/management/catalog"
	"github.com/rancher/rancher/pkg/controllers/management/clusterevents"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstats"
	"github.com/rancher/rancher/pkg/dialer"
	machineController "github.com/rancher/rancher/pkg/machine/controller"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext, dialerFactory dialer.Factory) {
	// auth handlers need to run early to create namespaces that back clusters and projects
	// also, these handlers are purely in the mgmt plane, so they are lightweight compared to those that interact with machines and clusters
	auth.RegisterEarly(ctx, management)
	agent.Register(ctx, management)
	machineController.Register(management)
	catalog.Register(ctx, management)
	clusterstats.Register(management)
	clusterevents.Register(ctx, management)
	clusterprovisioner.Register(management, dialerFactory)
	auth.RegisterLate(ctx, management)
	registerClusterScopedGC(ctx, management)
}
