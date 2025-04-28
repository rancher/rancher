package project_cluster

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	// add indexer to project resources.
	management.Wrangler.Mgmt.Project().Cache().AddIndexer(RTPrtbIndex, PRIndexer)
	enqueuer := ProjectClusterenqueuer{
		RTCache: management.Wrangler.Mgmt.Project().Cache(),
	}
	// this will enqueue Projects when a ProjectRoleTemplateBinding changes.
	// this is needed by checkPSAMembershipRole in order to list all the roletemplates when projects are created.
	relatedresource.Watch(ctx, "prtb-watcher", enqueuer.EnqueueRoleTemplates, management.Wrangler.Mgmt.Project(), management.Wrangler.Mgmt.ProjectRoleTemplateBinding())

	p := NewProjectLifecycle(management)
	c := NewClusterLifecycle(management)
	management.Management.Projects("").AddLifecycle(ctx, ProjectRemoveController, p)
	management.Management.Clusters("").AddLifecycle(ctx, ClusterRemoveController, c)
}
