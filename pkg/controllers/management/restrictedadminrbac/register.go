package restrictedadminrbac

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/pkg/relatedresource"
	"k8s.io/client-go/tools/cache"
)

type rbaccontroller struct {
	grbIndexer         cache.Indexer
	globalRoleBindings v3.GlobalRoleBindingInterface
	roleBindings       v1.RoleBindingInterface
	rbLister           v1.RoleBindingLister
	fleetworkspaces    v3.FleetWorkspaceInterface
}

const (
	grbByRoleIndex = "management.cattle.io/grb-by-role"
)

func Register(ctx context.Context, management *config.ManagementContext, wrangler *wrangler.Context) {

	informer := management.Management.GlobalRoleBindings("").Controller().Informer()
	r := rbaccontroller{
		globalRoleBindings: management.Management.GlobalRoleBindings(""),
		grbIndexer:         informer.GetIndexer(),
		roleBindings:       management.RBAC.RoleBindings(""),
		rbLister:           management.RBAC.RoleBindings("").Controller().Lister(),
		fleetworkspaces:    management.Management.FleetWorkspaces(""),
	}

	r.globalRoleBindings.AddHandler(ctx, "restrictedAdminGlobalBindingsFleet", r.ensureRestricedAdminForFleet)
	relatedresource.Watch(ctx, "restricted-admin-fleet", r.enqueueGrb, r.globalRoleBindings.Controller(), wrangler.Mgmt.FleetWorkspace())

}
