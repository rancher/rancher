package restrictedadminrbac

import (
	"context"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	v1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"k8s.io/client-go/tools/cache"
)

type rbaccontroller struct {
	grbIndexer      cache.Indexer
	roleBindings    v1.RoleBindingInterface
	rbLister        v1.RoleBindingLister
	fleetworkspaces v3.FleetWorkspaceInterface
	clusterCache    mgmtv3.ClusterCache
	grbCache        mgmtv3.GlobalRoleBindingCache
	crtbCache       mgmtv3.ClusterRoleTemplateBindingCache
	crtbCtrl        mgmtv3.ClusterRoleTemplateBindingClient
}

const (
	grbByRoleIndex = "management.cattle.io/grb-by-role"
	sourceKey      = "field.cattle.io/source"
	grbHandlerName = "restrictedAdminsClusterOwner"
)

func Register(ctx context.Context, management *config.ManagementContext, wrangler *wrangler.Context) {

	informer := management.Management.GlobalRoleBindings("").Controller().Informer()
	globalRoleBindings := management.Management.GlobalRoleBindings("")
	r := rbaccontroller{
		grbIndexer:      informer.GetIndexer(),
		roleBindings:    management.RBAC.RoleBindings(""),
		rbLister:        management.RBAC.RoleBindings("").Controller().Lister(),
		fleetworkspaces: management.Management.FleetWorkspaces(""),
		clusterCache:    management.Wrangler.Mgmt.Cluster().Cache(),
		crtbCache:       management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		crtbCtrl:        management.Wrangler.Mgmt.ClusterRoleTemplateBinding(),
		grbCache:        management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
	}
	globalRoleBindings.AddHandler(ctx, grbHandlerName, r.clusterOwnerSync)
	globalRoleBindings.AddHandler(ctx, "restrictedAdminGlobalBindingsFleet", r.ensureRestricedAdminForFleet)

	// if a fleetwokspace changes then enqueue the restricted-admin globalRoleBindings
	relatedresource.Watch(ctx, "restricted-admin-fleet", r.enqueueGrb, globalRoleBindings.Controller(), wrangler.Mgmt.FleetWorkspace())

	// if a cluster is updated enqueue the restricted-admin globalRoleBindings
	relatedresource.Watch(ctx, "restricted-admin-cluster", r.enqueueGrbOnCluster, globalRoleBindings.Controller(), wrangler.Mgmt.Cluster())

	// if a CRTB that is owned by a GRB is modified enqueue the GRB to reconcile the modified CRTB.
	relatedresource.Watch(ctx, "restricted-admin-crtb", r.enqueueGrbOnCRTB, globalRoleBindings.Controller(), wrangler.Mgmt.ClusterRoleTemplateBinding())
}

// getRestrictedAdminGRBs gets returns a list of keys for all restricted admin GlobalRoleBindings.
func (r *rbaccontroller) getRestrictedAdminGRBs() ([]relatedresource.Key, error) {
	grbs, err := r.grbIndexer.ByIndex(grbByRoleIndex, rbac.GlobalRestrictedAdmin)
	if err != nil {
		return nil, err
	}

	result := make([]relatedresource.Key, 0, len(grbs))
	for _, grbObj := range grbs {
		grb, ok := grbObj.(*v3.GlobalRoleBinding)
		if !ok {
			continue
		}
		result = append(result, relatedresource.Key{
			Namespace: grb.Namespace,
			Name:      grb.Name,
		})
	}

	return result, nil
}
