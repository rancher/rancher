package management

import (
	"context"

	"github.com/rancher/rancher/pkg/multiclustermanager/clustermanager"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/auth"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/catalog"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/certsexpiration"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/cis"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/cloudcredential"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/cluster"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clusterdeploy"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clustergc"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clusterprovisioner"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clusterregistrationtoken"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clusterstats"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clusterstatus"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/clustertemplate"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/compose"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/drivers/kontainerdriver"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/drivers/nodedriver"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/etcdbackup"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/globaldns"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/kontainerdrivermetadata"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/multiclusterapp"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/node"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/nodepool"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/nodetemplate"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/podsecuritypolicy"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/rbac"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/rkeworkerupgrader"
	"github.com/rancher/rancher/pkg/multiclustermanager/controllers/management/usercontrollers"
	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext, manager *clustermanager.Manager) {
	// auth handlers need to run early to create namespaces that back clusters and projects
	// also, these handlers are purely in the mgmt plane, so they are lightweight compared to those that interact with machines and clusters
	auth.RegisterEarly(ctx, management, manager)
	usercontrollers.RegisterEarly(ctx, management, manager)

	// a-z
	catalog.Register(ctx, management)
	certsexpiration.Register(ctx, management)
	cluster.Register(ctx, management)
	clusterdeploy.Register(ctx, management, manager)
	clustergc.Register(ctx, management)
	clusterprovisioner.Register(ctx, management)
	clusterstats.Register(ctx, management, manager)
	clusterstatus.Register(ctx, management)
	clusterregistrationtoken.Register(ctx, management)
	compose.Register(ctx, management, manager)
	kontainerdriver.Register(ctx, management)
	kontainerdrivermetadata.Register(ctx, management)
	nodedriver.Register(ctx, management)
	nodepool.Register(ctx, management)
	cloudcredential.Register(ctx, management)
	node.Register(ctx, management, manager)
	podsecuritypolicy.Register(ctx, management)
	etcdbackup.Register(ctx, management)
	cis.Register(ctx, management)
	globaldns.Register(ctx, management)
	multiclusterapp.Register(ctx, management, manager)
	clustertemplate.Register(ctx, management)
	nodetemplate.Register(ctx, management)
	rkeworkerupgrader.Register(ctx, management, manager.ScaledContext)
	rbac.Register(ctx, management)

	// Register last
	auth.RegisterLate(ctx, management)

	// Ensure caches are available for user controllers, these are used as part of
	// registration
	management.Management.ClusterAlertGroups("").Controller()
	management.Management.ClusterAlertRules("").Controller()

}
