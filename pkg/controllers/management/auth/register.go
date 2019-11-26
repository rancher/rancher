package auth

import (
	"context"

	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/types/config"
)

func RegisterEarly(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	prtb, crtb := newRTBLifecycles(management)
	gr := newGlobalRoleLifecycle(management)
	grb := newGlobalRoleBindingLifecycle(management, clusterManager)
	p, c := newPandCLifecycles(management)
	u := newUserLifecycle(management, clusterManager)
	n := newTokenController(management)
	ua := newUserAttributeController(management)
	s := newAuthSettingController(management)
	rt := newRoleTemplateHandler(management)
	grbLegacy := newLegacyGRBCleaner(management)
	rtLegacy := newLegacyRTCleaner(management)

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
	management.Management.RoleTemplates("").AddHandler(ctx, roleTemplateHandlerName, rt.sync)
	management.Management.GlobalRoleBindings("").AddHandler(ctx, "legacy-grb-cleaner", grbLegacy.sync)
	management.Management.RoleTemplates("").AddHandler(ctx, "legacy-rt-cleaner", rtLegacy.sync)
}

func RegisterLate(ctx context.Context, management *config.ManagementContext) {
	p, c := newPandCLifecycles(management)
	management.Management.Projects("").AddLifecycle(ctx, projectRemoveController, p)
	management.Management.Clusters("").AddLifecycle(ctx, clusterRemoveController, c)
}
