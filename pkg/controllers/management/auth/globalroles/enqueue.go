package globalroles

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	wrangler "github.com/rancher/wrangler/v2/pkg/name"
	"github.com/rancher/wrangler/v2/pkg/relatedresource"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	grbGrIndex                 = "mgmt-auth-grb-gr-idex"
	grNsIndex                  = "mgmt-auth-gr-ns-index"
	grSafeConcatIndex          = "mgmt-auth-gr-concat-index"
	grbSafeConcatIndex         = "mgmt-auth-grb-concat-index"
	grbEnqueuer                = "mgmt-auth-gr-enqueue"
	clusterGrEnqueuer          = "mgmt-auth-cluster-gr"
	crtbGRBEnqueuer            = "mgmt-auth-crtb-grb"
	roleEnqueuer               = "mgmt-auth-role-gr"
	roleBindingEnqueuer        = "mgmt-auth-rb-grb"
	namespaceGrEnqueuer        = "mgmt-auth-ns-gr"
	fleetWorkspaceGrbEnqueuer  = "mgmt-auth-fw-grb"
	clusterRoleEnqueuer        = "mgmt-auth-cr-gr"
	clusterRoleBindingEnqueuer = "mgmt-auth-crb-grb"
)

type globalRBACEnqueuer struct {
	grbCache      mgmtv3.GlobalRoleBindingCache
	grCache       mgmtv3.GlobalRoleCache
	clusterClient mgmtv3.ClusterClient
}

// grNsIndexer indexes GlobalRoles by the namespaces in NamespacedRules
func grNsIndexer(gr *v3.GlobalRole) ([]string, error) {
	result := []string{}
	for ns := range gr.NamespacedRules {
		result = append(result, ns)
	}
	return result, nil
}

// grbGrIndexer indexes a globalRoleBinding by the globalRole it assigns to users
func grbGrIndexer(grb *v3.GlobalRoleBinding) ([]string, error) {
	return []string{grb.GlobalRoleName}, nil
}

// grSafeConcatIndexer indexes a GlobalRole by the SafeConcat version of it's name
func grSafeConcatIndexer(gr *v3.GlobalRole) ([]string, error) {
	return []string{wrangler.SafeConcatName(gr.Name)}, nil
}

// grbSafeConcatIndexer indexes a GlobalRoleBinding by the SafeConcat version of it's name
func grbSafeConcatIndexer(grb *v3.GlobalRoleBinding) ([]string, error) {
	return []string{wrangler.SafeConcatName(grb.Name)}, nil
}

// enqueueGRBs enqueues GlobalRoleBinding for a given changed GlobalRole, allowing per-cluster permissions to sync
func (g *globalRBACEnqueuer) enqueueGRBs(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	globalRole, ok := obj.(*v3.GlobalRole)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a global role", obj)
		return nil, nil
	}
	bindings, err := g.grbCache.GetByIndex(grbGrIndex, globalRole.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get grbs for gr %s from indexer: %w", globalRole.Name, err)
	}
	bindingNames := make([]relatedresource.Key, 0, len(bindings))
	for _, binding := range bindings {
		bindingNames = append(bindingNames, relatedresource.Key{Name: binding.Name})
	}
	return bindingNames, nil
}

// clusterEnqueueGRs enqueues GlobalRoles which provide cluster RBAC. Does not enqueue any GRs if this cluster has already had
// the initial RBAC sync done
func (g *globalRBACEnqueuer) clusterEnqueueGRs(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	cluster, ok := obj.(*v3.Cluster)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type %[1]T to a cluster", obj)
		return nil, nil
	}
	// we only want to perform the initial sync once. Future changes will be picked up by other handlers
	if _, ok := cluster.Annotations[initialSyncAnnotation]; ok {
		return nil, nil
	}
	globalRoles, err := g.grCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list current GlobalRoles when syncing roles for cluster %s: %w", cluster.Name, err)
	}
	var rolesToSync []relatedresource.Key
	for _, globalRole := range globalRoles {
		if len(globalRole.InheritedClusterRoles) == 0 {
			continue
		}
		// Set the status as InProgress since we have to reconcile the global role
		globalRole.Status.Summary = SummaryInProgress
		rolesToSync = append(rolesToSync, relatedresource.Key{Name: globalRole.Name})
	}
	// attempt to update the cluster with a sync annotation - this is costly since it will re-enqueue all grs
	// which inherit cluster permissions, so we try to avoid it. If we can't record the annotation, we still
	// want to try and sync the permissions.
	newCluster, err := g.clusterClient.Get(cluster.Name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("unable to get cluster %s to add sync annotation, grs will re-enqueue on change: %s", cluster.Name, err.Error())
		return rolesToSync, nil
	}
	if newCluster.Annotations == nil {
		newCluster.Annotations = map[string]string{}
	}
	newCluster.Annotations[initialSyncAnnotation] = "true"
	_, err = g.clusterClient.Update(newCluster)
	if err != nil {
		logrus.Errorf("unable to update cluster %s with sync annotation, grs will re-enqueue on change: %s", cluster.Name, err.Error())
	}
	return rolesToSync, nil
}

// crtbEnqueueGRB enqueues GlobalRoleBindings which own a given CRTB when that CRTB is changed. Uses the label
// which is protected by the webhook rather than the ownerReference
func (g *globalRBACEnqueuer) crtbEnqueueGRB(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a crtb", obj)
		return nil, nil
	}
	grbOwner, ok := crtb.Labels[grbOwnerLabel]
	if !ok {
		// this crtb isn't owned by a GRB, no need to enqueue a GRB
		return nil, nil
	}
	_, err := g.grbCache.Get(grbOwner)
	if err != nil {
		// if the crtb was orphaned during deletion, the label may still exist but the owning grb won't
		// in these cases, nothing should be re-enqueued
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("unable to confirm if grb %s exists for crtb %s", grbOwner, crtb.Name)
	}
	return []relatedresource.Key{
		{Name: grbOwner},
	}, nil
}

// roleEnqueueGR enqueues GlobalRoles that own a given Role when that Role is changed. Uses grOwnerLabel
// which is protected by the webhook rather than the ownerReference.
func (g *globalRBACEnqueuer) roleEnqueueGR(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	role, ok := obj.(*v1.Role)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a Role", obj)
		return nil, nil
	}
	grOwner, ok := role.Labels[grOwnerLabel]
	if !ok {
		// this Role isn't owned by a GR, no need to enqueue a GR
		return nil, nil
	}
	grs, err := g.grCache.GetByIndex(grSafeConcatIndex, grOwner)
	if err != nil {
		return nil, fmt.Errorf("unable to get GlobalRole %s for Role %s", grOwner, role.Name)
	}

	grNames := make([]relatedresource.Key, 0, len(grs))
	for _, gr := range grs {
		// Set the status as InProgress since we have to reconcile the global role
		gr.Status.Summary = SummaryInProgress
		grNames = append(grNames, relatedresource.Key{Name: gr.Name})
	}
	return grNames, nil
}

// roleEnqueueGRB enqueues GlobalRoleBindings that own a given RoleBinding when that RoleBinding is changed.
// Uses grbOwnerLabel which is protected by the webhook rather than the ownerReference
func (g *globalRBACEnqueuer) roleBindingEnqueueGRB(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	roleBinding, ok := obj.(*v1.RoleBinding)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a RoleBinding", obj)
		return nil, nil
	}
	grbOwner, ok := roleBinding.Labels[grbOwnerLabel]
	if !ok {
		// this RoleBinding isn't owned by a GRB, no need to enqueue a GRB
		return nil, nil
	}
	grbs, err := g.grbCache.GetByIndex(grbSafeConcatIndex, grbOwner)
	if err != nil {
		return nil, fmt.Errorf("unable to get GlobalRoleBinding %s for RoleBinding %s", grbOwner, roleBinding.Name)
	}
	grbNames := make([]relatedresource.Key, 0, len(grbs))
	for _, grb := range grbs {
		grbNames = append(grbNames, relatedresource.Key{Name: grb.Name})
	}
	return grbNames, nil

}

// namespaceEnqueueGR enqueues GlobalRoles that have Roles in the changed namespace based on
// the NamespacedRules field
func (g *globalRBACEnqueuer) namespaceEnqueueGR(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a Namespace", obj)
		return nil, nil
	}

	grs, err := g.grCache.GetByIndex(grNsIndex, namespace.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to get grs for namespace %s from indexer: %w", namespace.Name, err)
	}
	grNames := make([]relatedresource.Key, 0, len(grs))
	for _, gr := range grs {
		// Set the status as InProgress since we have to reconcile the global role
		gr.Status.Summary = SummaryInProgress
		grNames = append(grNames, relatedresource.Key{Name: gr.Name})
	}
	return grNames, nil
}

// fleetWorkspaceEnqueueGRB enqueues GlobalRole that have set InheritedFleetWorkspacePermissions
// when a FleetWorkspace has changed.
func (g *globalRBACEnqueuer) fleetWorkspaceEnqueueGR(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}

	grs, err := g.grCache.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("unable to list current GlobalRoles: %w", err)
	}
	var grToSync []relatedresource.Key
	for _, gr := range grs {
		if gr.InheritedFleetWorkspacePermissions.WorkspaceVerbs != nil ||
			gr.InheritedFleetWorkspacePermissions.ResourceRules != nil {
			// Set the status as InProgress since we have to reconcile the global role
			gr.Status.Summary = SummaryInProgress
			grToSync = append(grToSync, relatedresource.Key{Name: gr.Name})
		}
	}

	return grToSync, nil
}

// clusterRoleEnqueueGRB enqueues GlobalRole when a generated ClusterRole changes.
func (g *globalRBACEnqueuer) clusterRoleEnqueueGR(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	clusterRole, ok := obj.(*v1.ClusterRole)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a ClusterRole", obj)
		return nil, nil
	}
	grOwner, ok := clusterRole.Labels[grOwnerLabel]
	if !ok {
		// this RoleBinding isn't owned by a GRB, no need to enqueue a GRB
		return nil, nil
	}
	grs, err := g.grCache.GetByIndex(grSafeConcatIndex, grOwner)
	if err != nil {
		return nil, fmt.Errorf("unable to get GlobalRole %s for RoleBinding %s: %w", grOwner, clusterRole.Name, err)
	}
	grNames := make([]relatedresource.Key, 0, len(grs))
	for _, gr := range grs {
		// Set the status as InProgress since we have to reconcile the global role
		gr.Status.Summary = SummaryInProgress
		grNames = append(grNames, relatedresource.Key{Name: gr.Name})
	}

	return grNames, nil
}

// clusterRoleBindingEnqueueGRB enqueues GlobalRoleBinding when a generated ClusterRoleBinding changes.
func (g *globalRBACEnqueuer) clusterRoleBindingEnqueueGRB(_, _ string, obj runtime.Object) ([]relatedresource.Key, error) {
	if obj == nil {
		return nil, nil
	}
	clusterRoleBinding, ok := obj.(*v1.ClusterRoleBinding)
	if !ok {
		logrus.Errorf("unable to convert object: %[1]v, type: %[1]T to a ClusterRole", obj)
		return nil, nil
	}
	grbOwner, ok := clusterRoleBinding.Labels[grbOwnerLabel]
	if !ok {
		// this RoleBinding isn't owned by a GRB, no need to enqueue a GRB
		return nil, nil
	}
	grbs, err := g.grbCache.GetByIndex(grbSafeConcatIndex, grbOwner)
	if err != nil {
		return nil, fmt.Errorf("unable to get GlobalRoleBinding %s for ClusterRoleBinding %s: %w", grbOwner, clusterRoleBinding.Name, err)
	}
	grbNames := make([]relatedresource.Key, 0, len(grbs))
	for _, grb := range grbs {
		grbNames = append(grbNames, relatedresource.Key{Name: grb.Name})
	}

	return grbNames, nil
}
