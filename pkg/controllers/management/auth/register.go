package auth

import (
	"context"

	"github.com/rancher/types/config"
)

func RegisterEarly(ctx context.Context, management *config.ManagementContext) {
	prtb, crtb := newRTBLifecycles(management)
	gr := newGlobalRoleLifecycle(management)
	grb := newGlobalRoleBindingLifecycle(management)
	p, c := newPandCLifecycles(management)
	u := newUserLifecycle(management)
	n := newTokenController(management)
	ua := newUserAttributeController(management)
	s := newAuthSettingController(management)

	management.Management.ProjectRoleTemplateBindings("").AddLifecycle(ctx, ptrbMGMTController, prtb)
	management.Management.ClusterRoleTemplateBindings("").AddLifecycle(ctx, ctrbMGMTController, crtb)
	management.Management.GlobalRoles("").AddLifecycle(ctx, grController, gr)
	management.Management.GlobalRoleBindings("").AddLifecycle(ctx, grbController, grb)
	management.Management.Projects("").AddHandler(ctx, projectCreateController, p.sync)
	management.Management.Clusters("").AddHandler(ctx, clusterCreateController, c.sync)
	management.Management.Users("").AddLifecycle(ctx, userController, u)
	management.Management.Tokens("").AddHandler(ctx, tokenController, n.sync)
	management.Management.UserAttributes("").AddHandler(ctx, userAttributeController, ua.sync)
	management.Management.Settings("").AddHandler(ctx, authSettingController, s.sync)
}

func RegisterLate(ctx context.Context, management *config.ManagementContext) {
	p, c := newPandCLifecycles(management)
	management.Management.Projects("").AddLifecycle(ctx, projectRemoveController, p)
	management.Management.Clusters("").AddLifecycle(ctx, clusterRemoveController, c)
}
