package roletemplates

import (
	"context"
	"fmt"

	"github.com/rancher/rancher/pkg/clustermanager"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	roleTemplateChangeHandler = "mgmt-roletemplate-change-handler"
	roleTemplateRemoveHandler = "mgmt-roletemplate-remove-handler"

	crtbChangeHandler        = "mgmt-crtb-change-handler"
	crtbRemoveHandler        = "mgmt-crtb-remove-handler"
	crtbRoleTemplateEnqueuer = "cluster-crtb-roletemplate-enqueuer"

	prtbChangeHandler        = "mgmt-prtb-change-handler"
	prtbRemoveHandler        = "mgmt-prtb-remove-handler"
	prtbRoleTemplateEnqueuer = "cluster-prtb-roletemplate-enqueuer"
)

func Register(ctx context.Context, management *config.ManagementContext, clusterManager *clustermanager.Manager) {
	r := newRoleTemplateHandler(management.Wrangler, clusterManager)
	management.Wrangler.Mgmt.RoleTemplate().OnChange(ctx, roleTemplateChangeHandler, r.OnChange)
	management.Wrangler.Mgmt.RoleTemplate().OnRemove(ctx, roleTemplateRemoveHandler, r.OnRemove)

	c := newCRTBHandler(management)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnChange(ctx, crtbChangeHandler, c.OnChange)
	management.Wrangler.Mgmt.ClusterRoleTemplateBinding().OnRemove(ctx, crtbRemoveHandler, c.OnRemove)

	p := newPRTBHandler(management)
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
}

type roletemplateEnqueuer struct {
	clusterCache mgmtv3.ClusterCache
	crtbCache    mgmtv3.ClusterRoleTemplateBindingCache
	projectCache mgmtv3.ProjectCache
	prtbCache    mgmtv3.ProjectRoleTemplateBindingCache
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
