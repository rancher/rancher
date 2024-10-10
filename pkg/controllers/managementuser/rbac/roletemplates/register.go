package roletemplates

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, workload *config.UserContext) {
	management := workload.Management.WithAgent("rbac-role-templates")

	management.Management.ClusterRoleTemplateBindings("").AddLifecycle(ctx, "cluster-crtb-handler", newCRTBLifecycle(workload))

	rth := newRoleTemplateHandler(workload)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, "cluster-roletemplate-change-handler", rth.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, "cluster-roletemplate-remove-handler", rth.OnRemove)
}
