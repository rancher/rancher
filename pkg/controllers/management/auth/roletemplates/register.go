package roletemplates

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/types/config"
)

const (
	roleTemplateChangeHandler = "mgmt-roletemplate-change-handler"
	roleTemplateRemoveHandler = "mgmt-roletemplate-remove-handler"

	crtbChangeHandler = "mgmt-crtb-change-handler"
	crtbRemoveHandler = "mgmt-crtb-remove-handler"

	prtbChangeHandler = "mgmt-prtb-change-handler"
	prtbRemoveHandler = "mgmt-prtb-remove-handler"
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	r := newRoleTemplateHandler(management.Wrangler, clusterManager)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, r.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, roleTemplateRemoveHandler, r.OnRemove)

	c := newCRTBHandler(management)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, crtbChangeHandler, c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, crtbRemoveHandler, c.OnRemove)

	p := newPRTBHandler(management)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, prtbChangeHandler, p.OnChange)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnRemove(ctx, prtbRemoveHandler, p.OnRemove)
}
