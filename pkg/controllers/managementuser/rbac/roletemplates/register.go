package roletemplates

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, workload *config.UserContext) {
	management := workload.Management.WithAgent("rbac-role-templates")

	c := newCRTBHandler(workload)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, "cluster-crtb-change-handler", c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, "cluster-crtb-remove-handler", c.OnRemove)

	p := newPRTBHandler(workload)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, "cluster-prtb-change-handler", p.OnChange)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnRemove(ctx, "cluster-prtb-remove-handler", p.OnRemove)

	rth := newRoleTemplateHandler(workload)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, "cluster-roletemplate-change-handler", rth.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, "cluster-roletemplate-remove-handler", rth.OnRemove)
}
