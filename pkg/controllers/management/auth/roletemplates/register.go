package roletemplates

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/clustermanager"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	prtbByUsernameIndex         = "auth.management.cattle.io/prtb-by-username"
	crtbByUsernameIndex         = "auth.management.cattle.io/crtb-by-username"
	rbByPRTBOwnerReferenceIndex = "auth.management.cattle.io/rb-by-prtb-owner-reference"
	rbByCRTBOwnerReferenceIndex = "auth.management.cattle.io/rb-by-crtb-owner-reference"
	prtbByRoleTemplateNameIndex = "auth.management.cattle.io/prtb-by-roletemplate-name"
	crtbByRoleTemplateNameIndex = "auth.management.cattle.io/crtb-by-roletemplate-name"

	roleTemplateChangeHandler = "mgmt-roletemplate-change-handler"
	roleTemplateRemoveHandler = "mgmt-roletemplate-remove-handler"

	crtbChangeHandler        = "mgmt-crtb-change-handler"
	crtbRemoveHandler        = "mgmt-crtb-remove-handler"
	crtbRoleTemplateEnqueuer = "cluster-crtb-roletemplate-enqueuer"
	crtbClusterRoleEnqueuer  = "cluster-crtb-clusterrole-enqueuer"

	prtbChangeHandler        = "mgmt-prtb-change-handler"
	prtbRemoveHandler        = "mgmt-prtb-remove-handler"
	prtbRoleTemplateEnqueuer = "cluster-prtb-roletemplate-enqueuer"
	prtbClusterRoleEnqueuer  = "cluster-prtb-clusterrole-enqueuer"
)

func RegisterIndexers(wranglerContext *wrangler.Context) {
	wranglerContext.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crtbByUsernameIndex, getCRTBByUsername)
	wranglerContext.Mgmt.ProjectRoleTemplateBinding().Cache().AddIndexer(prtbByUsernameIndex, getPRTBByUsername)
	wranglerContext.RBAC.RoleBinding().Cache().AddIndexer(rbByPRTBOwnerReferenceIndex, getRBByPRTBOwnerReference)
	wranglerContext.RBAC.RoleBinding().Cache().AddIndexer(rbByCRTBOwnerReferenceIndex, getRBByCRTBOwnerReference)
	wranglerContext.Mgmt.ProjectRoleTemplateBinding().Cache().AddIndexer(prtbByRoleTemplateNameIndex, getPRTBByRoleTemplateName)
	wranglerContext.Mgmt.ClusterRoleTemplateBinding().Cache().AddIndexer(crtbByRoleTemplateNameIndex, getCRTBByRoleTemplateName)
}

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	r := newRoleTemplateHandler(management.Wrangler, clusterManager)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, r.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, roleTemplateRemoveHandler, r.OnRemove)

	c := newCRTBHandler(management, clusterManager)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, crtbChangeHandler, c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, crtbRemoveHandler, c.OnRemove)

	p := newPRTBHandler(management, clusterManager)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnChange(ctx, prtbChangeHandler, p.OnChange)
	management.Wrangler.Mgmt.ProjectRoleTemplateBinding().OnRemove(ctx, prtbRemoveHandler, p.OnRemove)

	// Add resource watchers
	roletemplateEnqueuer := &roletemplateEnqueuer{
		clusterCache: management.Wrangler.Mgmt.Cluster().Cache(),
		crtbCache:    management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
		prtbCache:    management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		projectCache: management.Wrangler.Mgmt.Project().Cache(),
	}
	relatedresource.Watch(ctx, crtbRoleTemplateEnqueuer, roletemplateEnqueuer.roletemplateEnqueueCRTBs, management.Wrangler.Mgmt.ClusterRoleTemplateBinding(), management.Wrangler.Mgmt.RoleTemplate())
	relatedresource.Watch(ctx, prtbRoleTemplateEnqueuer, roletemplateEnqueuer.roletemplateEnqueuePRTBs, management.Wrangler.Mgmt.ProjectRoleTemplateBinding(), management.Wrangler.Mgmt.RoleTemplate())
	relatedresource.Watch(ctx, crtbClusterRoleEnqueuer, roletemplateEnqueuer.clusterRoleEnqueueCRTBs, management.Wrangler.Mgmt.ClusterRoleTemplateBinding(), management.Wrangler.RBAC.ClusterRole())
	relatedresource.Watch(ctx, prtbClusterRoleEnqueuer, roletemplateEnqueuer.clusterRoleEnqueuePRTBs, management.Wrangler.Mgmt.ProjectRoleTemplateBinding(), management.Wrangler.RBAC.ClusterRole())
}

type roletemplateEnqueuer struct {
	clusterCache mgmtv3.ClusterCache
	crtbCache    mgmtv3.ClusterRoleTemplateBindingCache
	projectCache mgmtv3.ProjectCache
	prtbCache    mgmtv3.ProjectRoleTemplateBindingCache
}

// clusterRoleEnqueue extracts the owner from an aggregation ClusterRole and calls lookupKeys to resolve the related resource keys.
func clusterRoleEnqueue(obj runtime.Object, lookupKeys func(owner string) ([]relatedresource.Key, error)) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	clusterRole, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return nil, fmt.Errorf("failed to convert object %T to *ClusterRole", obj)
	}

	// If it's not an aggregation cluster role, ignore it
	if clusterRole.AggregationRule == nil {
		return nil, nil
	}

	if owner, ok := clusterRole.Labels[rbac.ClusterRoleOwnerLabel]; ok {
		return lookupKeys(owner)
	}

	return nil, nil
}

// clusterRoleEnqueuePRTBs enqueues PRTBs when the aggregation ClusterRole owned by the PRTB is changed.
func (r *roletemplateEnqueuer) clusterRoleEnqueuePRTBs(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	return clusterRoleEnqueue(obj, func(owner string) ([]relatedresource.Key, error) {
		prtbs, err := r.prtbCache.GetByIndex(prtbByRoleTemplateNameIndex, owner)
		if err != nil {
			return nil, fmt.Errorf("failed to list PRTBs for role template %s: %w", owner, err)
		}
		var keys []relatedresource.Key
		for _, prtb := range prtbs {
			keys = append(keys, relatedresource.Key{
				Name:      prtb.Name,
				Namespace: prtb.Namespace,
			})
		}
		return keys, nil
	})
}

// clusterRoleEnqueueCRTBs enqueues CRTBs when the aggregation ClusterRole owned by the CRTB is changed.
func (r *roletemplateEnqueuer) clusterRoleEnqueueCRTBs(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	return clusterRoleEnqueue(obj, func(owner string) ([]relatedresource.Key, error) {
		crtbs, err := r.crtbCache.GetByIndex(crtbByRoleTemplateNameIndex, owner)
		if err != nil {
			return nil, fmt.Errorf("failed to list CRTBs for role template %s: %w", owner, err)
		}
		var keys []relatedresource.Key
		for _, crtb := range crtbs {
			keys = append(keys, relatedresource.Key{
				Name:      crtb.Name,
				Namespace: crtb.Namespace,
			})
		}
		return keys, nil
	})
}

// roletemplateEnqueuePRTBs enqueues PRTBs that reference the changed RoleTemplate.
func (r *roletemplateEnqueuer) roletemplateEnqueuePRTBs(_, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}

	clusters, err := r.clusterCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	var keys []relatedresource.Key
	for _, cluster := range clusters {
		projects, err := r.projectCache.List(cluster.Name, labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list projects: %w", err)
		}

		for _, project := range projects {
			prtbs, err := r.prtbCache.List(project.GetProjectBackingNamespace(), labels.Everything())
			if err != nil {
				return nil, fmt.Errorf("failed to list ProjectRoleTemplateBindings in namespace %s: %w", project.GetProjectBackingNamespace(), err)
			}
			for _, prtb := range prtbs {
				if prtb.RoleTemplateName == name {
					keys = append(keys, relatedresource.Key{
						Name:      prtb.Name,
						Namespace: prtb.Namespace,
					})
				}
			}
		}
	}

	return keys, nil
}

// roletemplateEnqueueCRTBs enqueues CRTBs that reference the changed RoleTemplate.
func (r *roletemplateEnqueuer) roletemplateEnqueueCRTBs(_, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}

	clusters, err := r.clusterCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	var keys []relatedresource.Key
	for _, cluster := range clusters {
		crtbs, err := r.crtbCache.List(cluster.Name, labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("failed to list ClusterRoleTemplateBindings: %w", err)
		}
		for _, crtb := range crtbs {
			if crtb.RoleTemplateName == name {
				keys = append(keys, relatedresource.Key{
					Name:      crtb.Name,
					Namespace: crtb.Namespace,
				})
			}
		}
	}

	return keys, nil
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

func getRBByPRTBOwnerReference(rb *rbacv1.RoleBinding) ([]string, error) {
	if rb == nil {
		return []string{}, nil
	}

	if rb.OwnerReferences != nil {
		for _, ownerRef := range rb.OwnerReferences {
			if ownerRef.Kind == "ProjectRoleTemplateBinding" {
				return []string{ownerRef.Name}, nil
			}
		}
	}
	return []string{}, nil
}

func getRBByCRTBOwnerReference(rb *rbacv1.RoleBinding) ([]string, error) {
	if rb == nil {
		return []string{}, nil
	}

	if rb.OwnerReferences != nil {
		for _, ownerRef := range rb.OwnerReferences {
			if ownerRef.Kind == "ClusterRoleTemplateBinding" {
				return []string{ownerRef.Name}, nil
			}
		}
	}
	return []string{}, nil
}

func getPRTBByRoleTemplateName(prtb *v3.ProjectRoleTemplateBinding) ([]string, error) {
	if prtb == nil {
		return []string{}, nil
	}
	if prtb.RoleTemplateName != "" {
		return []string{prtb.RoleTemplateName}, nil
	}
	return []string{}, nil
}

func getCRTBByRoleTemplateName(crtb *v3.ClusterRoleTemplateBinding) ([]string, error) {
	if crtb == nil {
		return []string{}, nil
	}
	if crtb.RoleTemplateName != "" {
		return []string{crtb.RoleTemplateName}, nil
	}
	return []string{}, nil
}
