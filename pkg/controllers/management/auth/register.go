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

	// todo: move into user controller
	management.Management.ProjectRoleTemplateBindings("").AddLifecycle(ptrbMGMTController, prtb)
	management.Management.ClusterRoleTemplateBindings("").AddLifecycle(ctrbMGMTController, crtb)
	management.Management.GlobalRoles("").AddLifecycle(grController, gr)
	management.Management.GlobalRoleBindings("").AddLifecycle(grbController, grb)
	management.Management.Projects("").AddHandler(projectCreateController, p.sync)
	management.Management.Clusters("").AddHandler(clusterCreateController, c.sync)
	management.Management.Users("").AddLifecycle(userController, u)
	management.Management.Tokens("").AddHandler(tokenController, n.sync)
}

func RegisterLate(ctx context.Context, management *config.ManagementContext) {
	p, c := newPandCLifecycles(management)
	management.Management.Projects("").AddLifecycle(projectRemoveController, p)
	management.Management.Clusters("").AddLifecycle(clusterRemoveController, c)
}
