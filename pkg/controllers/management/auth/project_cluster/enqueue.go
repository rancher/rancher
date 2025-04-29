package project_cluster

import (
	"context"
	"fmt"

	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/types/config"
	wrangler "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	RTPrtbIndex = "mgmt-auth-rt-prtb-idex"
)

type ProjectClusterenqueuer struct {
	RTCache mgmtv3.ProjectCache
}

// rtIndexer indexes a RoleTemplate
func PRIndexer(p *v3.Project) ([]string, error) {
	return []string{wrangler.SafeConcatName(p.Name)}, nil
}

// enqueueRoleTemplates enqueues RoleTemplates for a given changed ProjectRoleTemplateBinding, allowing per-cluster permissions to sync
func (p *ProjectClusterenqueuer) EnqueueRoleTemplates(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a projectroletemplatebinding", obj)
		return nil, nil
	}
	roleTemplates, err := p.RTCache.GetByIndex(RTPrtbIndex, prtb.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get roletemplates for projectroletemplatebinding %s from indexer: %w", prtb.Name, err)
	}
	roleTemplateNames := make([]relatedresource.Key, 0, len(roleTemplates))
	for _, binding := range roleTemplates {
		roleTemplateNames = append(roleTemplateNames, relatedresource.Key{Name: binding.Name})
	}
	return roleTemplateNames, nil
}

func Enqueue(ctx context.Context, management *config.ManagementContext) {
	// add indexer to project resources.
	management.Wrangler.Mgmt.Project().Cache().AddIndexer(RTPrtbIndex, PRIndexer)
	enqueuer := ProjectClusterenqueuer{
		RTCache: management.Wrangler.Mgmt.Project().Cache(),
	}
	// this will enqueue Projects when a ProjectRoleTemplateBinding changes.
	// this is needed by checkPSAMembershipRole in order to list all the roletemplates when projects are created.
	relatedresource.Watch(ctx, "prtb-watcher", enqueuer.EnqueueRoleTemplates, management.Wrangler.Mgmt.Project(), management.Wrangler.Mgmt.ProjectRoleTemplateBinding())
}
