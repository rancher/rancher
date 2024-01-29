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
	enqueuer := globalRBACEnqueuer{
		grbCache:      management.Wrangler.Mgmt.GlobalRoleBinding().Cache(),
		grCache:       management.Wrangler.Mgmt.GlobalRole().Cache(),
		clusterClient: management.Wrangler.Mgmt.Cluster(),
	}
	relatedresource.WatchClusterScoped(ctx, grbEnqueuer, enqueuer.enqueueGRBs, management.Wrangler.Mgmt.GlobalRoleBinding(), management.Wrangler.Mgmt.GlobalRole())
	relatedresource.WatchClusterScoped(ctx, clusterGrEnqueuer, enqueuer.clusterEnqueueGRs, management.Wrangler.Mgmt.GlobalRole(), management.Wrangler.Mgmt.Cluster())
	relatedresource.WatchClusterScoped(ctx, crtbGRBEnqueuer, enqueuer.crtbEnqueueGRB, management.Wrangler.Mgmt.GlobalRoleBinding(), management.Wrangler.Mgmt.ClusterRoleTemplateBinding())

	gr := newGlobalRoleLifecycle(management.WithAgent(grController))
	grb := newGlobalRoleBindingLifecycle(management.WithAgent(grbController), clusterManager)
	management.Management.GlobalRoles("").AddLifecycle(ctx, grController, gr)
	management.Management.GlobalRoleBindings("").AddLifecycle(ctx, grbController, grb)
}
