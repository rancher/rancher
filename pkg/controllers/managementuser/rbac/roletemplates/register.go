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
	corev1 "k8s.io/api/core/v1"
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

	// Namespace-side enqueuer that re-triggers the PRTB handler when a namespace in a project
	// changes. reconcileBindings creates the per-namespace RoleBindings by iterating the project's
	// namespaces, so a namespace created after its PRTB would otherwise never receive a binding.
	prtbNamespaceEnqueuer = "aggregation-prtb-namespace-enqueuer"
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
	relatedresource.Watch(ctx, prtbNamespaceEnqueuer, enqueuer.enqueuePRTBsForNamespace,
		management.Wrangler.Mgmt.ProjectRoleTemplateBinding(), workload.Corew.Namespace())

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
	// Use the cluster-scoped index so we only fetch the PRTBs belonging to this cluster, instead of
	// every PRTB referencing the RoleTemplate across all clusters and then filtering by clusterName.
	prtbs, err := e.prtbCache.GetByIndex(rbac.PRTBByClusterAndRoleTemplateNameIndex, rbac.RoleTemplateClusterIndexKey(e.clusterName, name))
	if err != nil {
		return nil, fmt.Errorf("failed to list ProjectRoleTemplateBindings for roletemplate %s: %w", name, err)
	}
	var keys []relatedresource.Key
	for _, prtb := range prtbs {
		keys = append(keys, relatedresource.Key{Namespace: prtb.Namespace, Name: prtb.Name})
	}
	return keys, nil
}

// enqueuePRTBsForNamespace enqueues every PRTB in the changed namespace's project so their
// per-namespace RoleBindings are reconciled. Without this, a namespace created after its PRTB never
// receives a binding, because reconcileBindings only runs on PRTB and RoleTemplate changes.
func (e *roleTemplateEnqueuer) enqueuePRTBsForNamespace(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	ns, ok := obj.(*corev1.Namespace)
	if !ok || ns == nil {
		return nil, nil
	}
	projectID := ns.Annotations[projectIDAnnotation]
	if projectID == "" {
		return nil, nil
	}
	prtbs, err := e.prtbCache.GetByIndex(rbac.PRTBByProjectNameIndex, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list ProjectRoleTemplateBindings for project %s: %w", projectID, err)
	}
	var keys []relatedresource.Key
	for _, prtb := range prtbs {
		keys = append(keys, relatedresource.Key{Namespace: prtb.Namespace, Name: prtb.Name})
	}
	return keys, nil
}

// enqueueCRTBs enqueues every CRTB in this cluster that references the changed RoleTemplate.
func (e *roleTemplateEnqueuer) enqueueCRTBs(_, name string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	// Use the cluster-scoped index so we only fetch the CRTBs belonging to this cluster, instead of
	// every CRTB referencing the RoleTemplate across all clusters and then filtering by clusterName.
	crtbs, err := e.crtbCache.GetByIndex(rbac.CRTBByClusterAndRoleTemplateNameIndex, rbac.RoleTemplateClusterIndexKey(e.clusterName, name))
	if err != nil {
		return nil, fmt.Errorf("failed to list ClusterRoleTemplateBindings for roletemplate %s: %w", name, err)
	}
	var keys []relatedresource.Key
	for _, crtb := range crtbs {
		keys = append(keys, relatedresource.Key{Namespace: crtb.Namespace, Name: crtb.Name})
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
