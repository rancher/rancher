package management

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/management/auth"
	"github.com/rancher/rancher/pkg/controllers/management/catalog"
	"github.com/rancher/rancher/pkg/controllers/management/clusterevents"
	"github.com/rancher/rancher/pkg/controllers/management/clustergc"
	"github.com/rancher/rancher/pkg/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/controllers/management/clusterstats"
	"github.com/rancher/rancher/pkg/controllers/management/compose"
	"github.com/rancher/rancher/pkg/controllers/management/node"
	"github.com/rancher/rancher/pkg/controllers/management/nodedriver"
	"github.com/rancher/rancher/pkg/controllers/management/nodepool"
	"github.com/rancher/rancher/pkg/controllers/management/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/controllers/management/usercontrollers"
	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	// auth handlers need to run early to create namespaces that back clusters and projects
	// also, these handlers are purely in the mgmt plane, so they are lightweight compared to those that interact with machines and clusters
	auth.RegisterEarly(ctx, management)
	usercontrollers.RegisterEarly(ctx, management, manager)

	// a-z
	catalog.Register(ctx, management)
	clusterevents.Register(ctx, management)
	clustergc.Register(management)
	clusterprovisioner.Register(management)
	clusterstats.Register(management, manager)
	compose.Register(management, manager)
	nodedriver.Register(management)
	nodepool.Register(management)
	node.Register(management)
	usercontrollers.RegisterLate(ctx, management, manager)
	podsecuritypolicy.Register(management)

	// Register last
	auth.RegisterLate(ctx, management)
}
