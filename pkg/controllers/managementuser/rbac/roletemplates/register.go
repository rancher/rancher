package roletemplates

import (
	"context"

	"github.com/rancher/rancher/pkg/types/config"
)

func Register(ctx context.Context, workload *config.UserContext) {
	management := workload.Management.WithAgent("rbac-role-templates")

	management.Management.RoleTemplates("").AddLifecycle(ctx, "cluster-roletemplate-handler", newRoleTemplateLifecycle(workload))
}
