package roletemplates

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v3/pkg/name"
	"github.com/rancher/wrangler/v3/pkg/relatedresource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	crtbChangeHandler         = "cluster-crtb-change-handler"
	prtbChangeHandler         = "cluster-prtb-change-handler"
	roleTemplateChangeHandler = "cluster-roletemplate-change-handler"

	// Owner-side enqueuers that re-trigger the binding handlers when a RoleTemplate changes.
	// These must be registered on the owner plane (alongside the handlers above) so that editing a
	// RoleTemplate enqueues the referencing PRTBs/CRTBs on the same workqueue the handlers run on.
	prtbRoleTemplateEnqueuer = "aggregation-prtb-roletemplate-enqueuer"
	crtbRoleTemplateEnqueuer = "aggregation-crtb-roletemplate-enqueuer"
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

	// Register owner-side enqueuers so that a RoleTemplate edit re-triggers the PRTB/CRTB handlers
	// above. The equivalent enqueuers in pkg/controllers/management/auth/roletemplates run on the
	// leader plane and only reach the leader-side handlers; in HA the owner is a different replica,
	// so without these the aggregated bindings would not reconcile when a RoleTemplate changes.
	enqueuer := &roleTemplateEnqueuer{
		clusterName: workload.ClusterName,
		prtbCache:   management.Wrangler.Mgmt.ProjectRoleTemplateBinding().Cache(),
		crtbCache:   management.Wrangler.Mgmt.ClusterRoleTemplateBinding().Cache(),
	}
	relatedresource.Watch(ctx, prtbRoleTemplateEnqueuer, enqueuer.enqueuePRTBs,
		management.Wrangler.Mgmt.ProjectRoleTemplateBinding(), management.Wrangler.Mgmt.RoleTemplate())
	relatedresource.Watch(ctx, crtbRoleTemplateEnqueuer, enqueuer.enqueueCRTBs,
		management.Wrangler.Mgmt.ClusterRoleTemplateBinding(), management.Wrangler.Mgmt.RoleTemplate())

	return nil
}

// roleTemplateEnqueuer resolves the PRTBs/CRTBs in this cluster that reference a changed RoleTemplate.
type roleTemplateEnqueuer struct {
	clusterName string
	prtbCache   mgmtv3.ProjectRoleTemplateBindingCache
	crtbCache   mgmtv3.ClusterRoleTemplateBindingCache
}

// enqueuePRTBs enqueues every PRTB in this cluster that references the changed RoleTemplate.
func (e *roleTemplateEnqueuer) enqueuePRTBs(_, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	prtbs, err := e.prtbCache.List(metav1.NamespaceAll, labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list ProjectRoleTemplateBindings: %w", err)
	}
	var keys []relatedresource.Key
	for _, prtb := range prtbs {
		if prtb.RoleTemplateName != name {
			continue
		}
		clusterName, _ := rbac.GetClusterAndProjectNameFromPRTB(prtb)
		if clusterName != e.clusterName {
			continue
		}
		keys = append(keys, relatedresource.Key{Namespace: prtb.Namespace, Name: prtb.Name})
	}
	return keys, nil
}

// enqueueCRTBs enqueues every CRTB in this cluster that references the changed RoleTemplate.
func (e *roleTemplateEnqueuer) enqueueCRTBs(_, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	crtbs, err := e.crtbCache.List(e.clusterName, labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list ClusterRoleTemplateBindings for cluster %s: %w", e.clusterName, err)
	}
	var keys []relatedresource.Key
	for _, crtb := range crtbs {
		if crtb.RoleTemplateName == name {
			keys = append(keys, relatedresource.Key{Namespace: crtb.Namespace, Name: crtb.Name})
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
