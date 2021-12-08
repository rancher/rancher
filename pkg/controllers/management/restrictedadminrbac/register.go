package restrictedadminrbac

import (
	"context"

	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/client-go/tools/cache"
)

type rbaccontroller struct {
	grbLister           v3.GlobalRoleBindingLister
	grbIndexer          cache.Indexer
	globalRoleBindings  v3.GlobalRoleBindingInterface
	roleBindings        v1.RoleBindingInterface
	rbLister            v1.RoleBindingLister
	clusters            v3.ClusterInterface
	projects            v3.ProjectInterface
	clusterRoles        v1.ClusterRoleInterface
	crLister            v1.ClusterRoleLister
	crbLister           v1.ClusterRoleBindingLister
	clusterRoleBindings v1.ClusterRoleBindingInterface
	fleetworkspaces     v3.FleetWorkspaceInterface
	provClusters        provisioningcontrollers.ClusterCache
}

const (
	grbByRoleIndex = "management.cattle.io/grb-by-role"
)

func Register(ctx context.Context, management *config.ManagementContext, wrangler *wrangler.Context) {

	informer := management.Management.GlobalRoleBindings("").Controller().Informer()
	r := rbaccontroller{
		clusters:            management.Management.Clusters(""),
		projects:            management.Management.Projects(""),
		grbLister:           management.Management.GlobalRoleBindings("").Controller().Lister(),
		globalRoleBindings:  management.Management.GlobalRoleBindings(""),
		grbIndexer:          informer.GetIndexer(),
		roleBindings:        management.RBAC.RoleBindings(""),
		rbLister:            management.RBAC.RoleBindings("").Controller().Lister(),
		crLister:            management.RBAC.ClusterRoles("").Controller().Lister(),
		clusterRoles:        management.RBAC.ClusterRoles(""),
		crbLister:           management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		clusterRoleBindings: management.RBAC.ClusterRoleBindings(""),
		fleetworkspaces:     management.Management.FleetWorkspaces(""),
		provClusters:        wrangler.Provisioning.Cluster().Cache(),
	}

	r.clusters.AddHandler(ctx, "restrictedAdminsRBACCluster", r.clusterRBACSync)
	r.projects.AddHandler(ctx, "restrictedAdminsRBACProject", r.projectRBACSync)

	r.globalRoleBindings.AddHandler(ctx, "restrictedAdminGlobalBindingsFleet", r.ensureRestricedAdminForFleet)
	relatedresource.Watch(ctx, "restricted-admin-fleet", r.enqueueGrb, r.globalRoleBindings.Controller(), wrangler.Mgmt.FleetWorkspace())

}
