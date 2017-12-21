package controller

import (
	"context"

	"github.com/rancher/types/config"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	prtb, crtb := newRTBLifecycles(management)
	gr := newGlobalRoleLifecycle(management)
	grb := newGlobalRoleBindingLifecycle(management)

	management.Management.ProjectRoleTemplateBindings("").AddLifecycle("mgmt-auth-prtb-controller", prtb)
	management.Management.ClusterRoleTemplateBindings("").AddLifecycle("mgmt-auth-crtb-controller", crtb)
	management.Management.GlobalRoles("").AddLifecycle("mgmt-auth-gr-controller", gr)
	management.Management.GlobalRoleBindings("").AddLifecycle("mgmt-auth-grb-controller", grb)
}
