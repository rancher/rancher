package roletemplates

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/name"
)

const (
	crtbChangeHandler         = "cluster-crtb-change-handler"
	prtbChangeHandler         = "cluster-prtb-change-handler"
	roleTemplateChangeHandler = "cluster-roletemplate-change-handler"
)

func Register(ctx context.Context, workload *config.UserContext) error {
	management := workload.Management.WithAgent("rbac-role-templates")

	c, err := newCRTBHandler(workload)
	if err != nil {
		return fmt.Errorf("cannot create clusterroletemplatebinding handler: %w", err)
	}
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, crtbChangeHandler, c.OnChange)

	p, err := newPRTBHandler(workload)
	if err != nil {
		return fmt.Errorf("cannot create projectroletemplatebinding handler: %w", err)
	}
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, prtbChangeHandler, p.OnChange)

	rth := newRoleTemplateHandler(workload)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, rth.OnChange)

	return nil
}

func getCRTBByUsername(crtb *v3.ClusterRoleTemplateBinding) ([]string, error) {
	if crtb == nil {
		return []string{}, nil
	}
	if crtb.UserName != "" && crtb.ClusterName != "" {
		return []string{name.SafeConcatName(crtb.ClusterName, crtb.UserName)}, nil
	}
	return []string{}, nil
}

func getPRTBByUsername(prtb *v3.ProjectRoleTemplateBinding) ([]string, error) {
	if prtb == nil {
		return []string{}, nil
	}
	if prtb.UserName != "" && prtb.ProjectName != "" {
		clusterName, _, _ := strings.Cut(prtb.ProjectName, ":")
		return []string{name.SafeConcatName(clusterName, prtb.UserName)}, nil
	}
	return []string{}, nil
}
