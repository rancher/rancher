package roletemplates

import (
	"context"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/name"
)

const (
	crtbChangeHandler   = "cluster-crtb-change-handler"
	crtbRemoveHandler   = "cluster-crtb-remove-handler"
	crtbUsernameIndexer = "cluster-crtb-username-indexer"

	prtbChangeHandler   = "cluster-prtb-change-handler"
	prtbRemoveHandler   = "cluster-prtb-remove-handler"
	prtbUsernameIndexer = "cluster-prtb-username-indexer"

	roleTemplateChangeHandler = "cluster-roletemplate-change-handler"
	roleTemplateRemoveHandler = "cluster-roletemplate-remove-handler"
)

func Register(ctx context.Context, workload *config.UserContext) {
	management := workload.Management.WithAgent("rbac-role-templates")

	c := newCRTBHandler(workload)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, crtbChangeHandler, c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, crtbRemoveHandler, c.OnRemove)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crtbUsernameIndexer, getCRTBByUsername)

	p := newPRTBHandler(workload)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, prtbChangeHandler, p.OnChange)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnRemove(ctx, prtbRemoveHandler, p.OnRemove)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache().AddIndexer(prtbUsernameIndexer, getPRTBByUsername)

	rth := newRoleTemplateHandler(workload)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, rth.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, roleTemplateRemoveHandler, rth.OnRemove)
}

func getCRTBByUsername(obj *v3.ClusterRoleTemplateBinding) ([]string, error) {
	if obj.UserName != "" && obj.ClusterName != "" {
		return []string{name.SafeConcatName(obj.ClusterName, obj.UserName)}, nil
	}
	return []string{}, nil
}

func getPRTBByUsername(obj *v3.ProjectRoleTemplateBinding) ([]string, error) {
	if obj.UserName != "" && obj.ProjectName != "" {
		clusterName, _, _ := strings.Cut(obj.ProjectName, ":")
		return []string{name.SafeConcatName(clusterName, obj.UserName)}, nil
	}
	return []string{}, nil
}
