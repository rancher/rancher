package roletemplates

import (
	"context"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/rancher/wrangler/v3/pkg/name"
	"k8s.io/apimachinery/pkg/runtime"
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
	scopedOnRemove(ctx, crtbRemoveHandler, management.Wrangler.Mgmt.ClusterRoleTemplateBinding(), crtbForCluster(workload.ClusterName), c.OnRemove)

	p := newPRTBHandler(workload)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, prtbChangeHandler, p.OnChange)
	scopedOnRemove(ctx, prtbRemoveHandler, management.Wrangler.Mgmt.ProjectRoleTemplateBinding(), prtbForCluster(workload.ClusterName), p.OnRemove)

	rth := newRoleTemplateHandler(workload)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, rth.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, roleTemplateRemoveHandler, rth.OnRemove)
}

// TODO(wrangler/v4): revert to use OnRemove when it supports options (https://github.com/rancher/wrangler/pull/472).
// OnRemove adds a wrapper around our handlers in order to manage finalizers.
// We need to filter objects outside of this wrapper if we are going to register the same handler for multiple user contexts
func scopedOnRemove[T generic.RuntimeMetaObject](ctx context.Context, name string, c generic.ControllerMeta, condition func(object runtime.Object) bool, sync generic.ObjectHandler[T]) {
	onRemoveHandler := generic.NewRemoveHandler(name, c.Updater(), generic.FromObjectHandlerToHandler(sync))
	c.AddGenericHandler(ctx, name, func(key string, obj runtime.Object) (runtime.Object, error) {
		if condition(obj) {
			return onRemoveHandler(key, obj)
		}
		return obj, nil
	})
}

func prtbForCluster(clusterName string) func(obj runtime.Object) bool {
	return func(obj runtime.Object) bool {
		if obj == nil {
			return false
		}
		prtbClusterName, _ := rbac.GetClusterAndProjectNameFromPRTB(obj.(*v3.ProjectRoleTemplateBinding))
		return clusterName == prtbClusterName
	}
}

func crtbForCluster(clusterName string) func(obj runtime.Object) bool {
	return func(obj runtime.Object) bool {
		if obj == nil {
			return false
		}
		return obj.(*v3.ClusterRoleTemplateBinding).ClusterName == clusterName
	}
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
