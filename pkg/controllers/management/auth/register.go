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

	management.Management.ProjectRoleTemplateBindings("").AddLifecycle("mgmt-auth-prtb-controller", prtb)
	management.Management.ClusterRoleTemplateBindings("").AddLifecycle("mgmt-auth-crtb-controller", crtb)
	management.Management.GlobalRoles("").AddLifecycle("mgmt-auth-gr-controller", gr)
	management.Management.GlobalRoleBindings("").AddLifecycle("mgmt-auth-grb-controller", grb)
	management.Management.Projects("").AddHandler("mgmt-project-rbac-create", p.sync)
	management.Management.Clusters("").AddHandler("mgmt-cluster-rbac-delete", c.sync)
	management.Management.Users("").AddLifecycle("mgmt-auth-users-controller", u)
	management.Management.Tokens("").AddHandler("mgmt-auth-tokens-controller", n.sync)
}

func RegisterLate(ctx context.Context, management *config.ManagementContext) {
	p, c := newPandCLifecycles(management)
	management.Management.Projects("").AddLifecycle("mgmt-project-rbac-remove", p)
	management.Management.Clusters("").AddLifecycle("mgmt-cluster-rbac-remove", c)
}
