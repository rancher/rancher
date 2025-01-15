package roletemplates

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	r := newRoleTemplateHandler(management.Wrangler)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, "mgmt-roletemplate-change-handler", r.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, "mgmt-roletemplate-remove-handler", r.OnRemove)

	c := newCRTBHandler(management)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, "mgmt-crtb-change-handler", c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, "mgmt-crtb-remove-handler", c.OnRemove)

	p := newPRTBHandler(management)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, "mgmt-prtb-change-handler", p.OnChange)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, "mgmt-prtb-remove-handler", p.OnRemove)
}
