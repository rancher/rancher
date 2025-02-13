package roletemplates

import (
	"context"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/name"
)

const (
	crtbChangeHandler   = "cluster-crtb-change-handler"
	crtbRemoveHandler   = "cluster-crtb-remove-handler"
	crtbByUsernameIndex = "auth.management.cattle.io/crtb-by-username"

	prtbChangeHandler   = "cluster-prtb-change-handler"
	prtbRemoveHandler   = "cluster-prtb-remove-handler"
	prtbByUsernameIndex = "auth.management.cattle.io/prtb-by-username"

	roleTemplateChangeHandler = "cluster-roletemplate-change-handler"
	roleTemplateRemoveHandler = "cluster-roletemplate-remove-handler"
)

func RegisterIndexers(wranglerContext *wrangler.Context) {
	wranglerContext.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crtbByUsernameIndex, getCRTBByUsername)
	wranglerContext.Mgmt.ProjectRoleTemplateBinding().Cache().AddIndexer(prtbByUsernameIndex, getPRTBByUsername)
}

func Register(ctx context.Context, workload *config.UserContext) {
	management := workload.Management.WithAgent("rbac-role-templates")

	c := newCRTBHandler(workload)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, crtbChangeHandler, c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, crtbRemoveHandler, c.OnRemove)

	p := newPRTBHandler(workload)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, prtbChangeHandler, p.OnChange)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnRemove(ctx, prtbRemoveHandler, p.OnRemove)

	rth := newRoleTemplateHandler(workload)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, rth.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, roleTemplateRemoveHandler, rth.OnRemove)
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
