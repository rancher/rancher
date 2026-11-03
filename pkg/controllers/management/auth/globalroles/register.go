package globalroles

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
)

const (
	grController  = "mgmt-auth-gr-controller"
	grbController = "mgmt-auth-grb-controller"
)

// RegisterWranglerIndexers registers the indexers that are also read outside of the leader
// replica. The controllers for a downstream cluster run on the replica that owns the cluster,
// which is not necessarily the leader, so indexers they depend on cannot be registered in
// Register.
func RegisterWranglerIndexers(grbCache mgmtv3.GlobalRoleBindingCache) {
	grbCache.AddIndexer(pkgrbac.GRBGlobalRoleIndex, grbGrIndexer)
}

// Register wires the GlobalRole and GlobalRoleBinding controllers, which run on the leader replica.
// RegisterWranglerIndexers has to be called first, since the enqueuers registered here read the
// indexers it adds.
func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	management.Wrangler.Mgmt.GlobalRoleBinding().Cache().AddIndexer(grbSafeConcatIndex, grbSafeConcatIndexer)
	management.Wrangler.Mgmt.GlobalRole().Cache().AddIndexer(grNsIndex, grNsIndexer)
	management.Wrangler.Mgmt.GlobalRole().Cache().AddIndexer(grSafeConcatIndex, grSafeConcatIndexer)
	enqueuer := globalRBACEnqueuer{
		grbCache:      management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		grCache:       management.Wrangler.Mgmt.GlobalRole().Cache(),
		clusterClient: management.Wrangler.Mgmt.Cluster(),
	}
	relatedresource.WatchClusterScoped(ctx, grbEnqueuer, enqueuer.enqueueGRBs, management.Wrangler.Mgmt.GlobalRoleBinding(), management.Wrangler.Mgmt.GlobalRole())
	relatedresource.WatchClusterScoped(ctx, clusterGrEnqueuer, enqueuer.clusterEnqueueGRs, management.Wrangler.Mgmt.GlobalRole(), management.Wrangler.Mgmt.Cluster())
	relatedresource.WatchClusterScoped(ctx, crtbGRBEnqueuer, enqueuer.crtbEnqueueGRB, management.Wrangler.Mgmt.GlobalRoleBinding(), management.Wrangler.Mgmt.ClusterRoleTemplateBinding())

	relatedresource.WatchClusterScoped(ctx, roleEnqueuer, enqueuer.roleEnqueueGR, management.Wrangler.Mgmt.GlobalRole(), management.Wrangler.RBAC.Role())
	relatedresource.WatchClusterScoped(ctx, roleBindingEnqueuer, enqueuer.roleBindingEnqueueGRB, management.Wrangler.Mgmt.GlobalRoleBinding(), management.Wrangler.RBAC.RoleBinding())
	relatedresource.WatchClusterScoped(ctx, namespaceGrEnqueuer, enqueuer.namespaceEnqueueGR, management.Wrangler.Mgmt.GlobalRole(), management.Wrangler.Core.Namespace())

	relatedresource.WatchClusterScoped(ctx, fleetWorkspaceGrbEnqueuer, enqueuer.fleetWorkspaceEnqueueGR, management.Wrangler.Mgmt.GlobalRole(), management.Wrangler.Mgmt.FleetWorkspace())
	relatedresource.WatchClusterScoped(ctx, clusterRoleEnqueuer, enqueuer.clusterRoleEnqueueGR, management.Wrangler.Mgmt.GlobalRole(), management.Wrangler.RBAC.ClusterRole())
	relatedresource.WatchClusterScoped(ctx, clusterRoleBindingEnqueuer, enqueuer.clusterRoleBindingEnqueueGRB, management.Wrangler.Mgmt.GlobalRoleBinding(), management.Wrangler.RBAC.ClusterRoleBinding())

	gr := newGlobalRoleLifecycle(management.WithAgent(grController), clusterManager)
	grb := newGlobalRoleBindingLifecycle(management.WithAgent(grbController), clusterManager)
	management.Management.GlobalRoles("").AddLifecycle(ctx, grController, gr)
	management.Management.GlobalRoleBindings("").AddLifecycle(ctx, grbController, grb)
}
