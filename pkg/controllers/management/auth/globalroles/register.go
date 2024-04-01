package globalroles

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
)

const (
	grController  = "mgmt-auth-gr-controller"
	grbController = "mgmt-auth-grb-controller"
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	management.Wrangler.Mgmt.GlobalRoleBinding().Cache().AddIndexer(grbGrIndex, grbGrIndexer)
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

	gr := newGlobalRoleLifecycle(management.WithAgent(grController))
	grb := newGlobalRoleBindingLifecycle(management.WithAgent(grbController), clusterManager)
	management.Management.GlobalRoles("").AddLifecycle(ctx, grController, gr)
	management.Management.GlobalRoleBindings("").AddLifecycle(ctx, grbController, grb)
}
