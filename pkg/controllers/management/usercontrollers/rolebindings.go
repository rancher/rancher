package usercontrollers

import (
	"fmt"
	"time"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	util "github.com/rancher/rancher/pkg/cluster"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	// Reasons for the RoleTemplateBindingsRemoved condition.
	reasonRemoving = "Removing"
	reasonRemoved  = "Removed"
	reasonTimedOut = "TimedOut"

	// roleTemplateBindingsRemovalTimeout is how long, from the start of the cluster's removal, its role
	// template bindings are given to be removed while the downstream cluster can still be reached. Like the
	// agent uninstall, which is tried 3 times, the removal then moves on.
	roleTemplateBindingsRemovalTimeout = 2 * time.Minute
)

// roleTemplateBindings are the clients removeRoleTemplateBindings needs.
type roleTemplateBindings struct {
	crtbs        mgmtcontrollers.ClusterRoleTemplateBindingClient
	crtbCache    mgmtcontrollers.ClusterRoleTemplateBindingCache
	prtbs        mgmtcontrollers.ProjectRoleTemplateBindingClient
	prtbCache    mgmtcontrollers.ProjectRoleTemplateBindingCache
	projectCache mgmtcontrollers.ProjectCache
}

// roleTemplateBindingsRemoval is the outcome of removeRoleTemplateBindings.
type roleTemplateBindingsRemoval struct {
	concluded bool
	status    coreV1.ConditionStatus
	reason    string
	message   string
}

// removeRoleTemplateBindings removes the ClusterRoleTemplateBindings and ProjectRoleTemplateBindings of
// cluster, which is being removed, while its downstream cluster can still be reached: their removal also
// removes what they granted there. It only does so for clusters that outlive their removal from Rancher, and
// only waits for them for a while. Projects are left alone: removing a project removes the namespaces Rancher
// created for it in the downstream cluster.
func (b *roleTemplateBindings) removeRoleTemplateBindings(cluster *v32.Cluster) (roleTemplateBindingsRemoval, error) {
	if !util.DownstreamCleanupRequired(cluster) || util.NeverConnected(cluster) {
		return roleTemplateBindingsRemoval{
			concluded: true,
			status:    coreV1.ConditionTrue,
			reason:    reasonNotRequired,
			message:   "what the cluster's role template bindings grant in the downstream cluster is removed with it, or was never deployed",
		}, nil
	}

	remaining, err := b.deleteRoleTemplateBindings(cluster)
	if err != nil {
		return roleTemplateBindingsRemoval{}, err
	}
	if remaining == 0 {
		return roleTemplateBindingsRemoval{concluded: true, status: coreV1.ConditionTrue, reason: reasonRemoved}, nil
	}

	if cluster.DeletionTimestamp != nil && time.Since(cluster.DeletionTimestamp.Time) > roleTemplateBindingsRemovalTimeout {
		logrus.Warnf("[cluster-cleanup] %d role template bindings of cluster [%s] were not removed within %s, moving on with removing the cluster", remaining, cluster.Name, roleTemplateBindingsRemovalTimeout)
		return roleTemplateBindingsRemoval{
			concluded: true,
			status:    coreV1.ConditionFalse,
			reason:    reasonTimedOut,
			message:   fmt.Sprintf("%d role template bindings were not removed within %s; what they grant in the downstream cluster is left in place", remaining, roleTemplateBindingsRemovalTimeout),
		}, nil
	}

	logrus.Debugf("[cluster-cleanup] waiting for %d role template bindings of cluster [%s] to be removed", remaining, cluster.Name)
	return roleTemplateBindingsRemoval{
		status:  coreV1.ConditionUnknown,
		reason:  reasonRemoving,
		message: "removing the cluster's role template bindings while the downstream cluster can be reached",
	}, nil
}

// deleteRoleTemplateBindings deletes the role template bindings of cluster that aren't being deleted yet, and
// returns how many are left.
func (b *roleTemplateBindings) deleteRoleTemplateBindings(cluster *v32.Cluster) (int, error) {
	crtbs, err := b.crtbCache.List(cluster.Name, labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("listing role template bindings of cluster %s: %w", cluster.Name, err)
	}
	remaining := len(crtbs)
	for _, crtb := range crtbs {
		if crtb.DeletionTimestamp != nil {
			continue
		}
		if err := b.crtbs.Delete(crtb.Namespace, crtb.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return 0, fmt.Errorf("deleting cluster role template binding %s/%s: %w", crtb.Namespace, crtb.Name, err)
		}
	}

	projects, err := b.projectCache.List(cluster.Name, labels.Everything())
	if err != nil {
		return 0, fmt.Errorf("listing projects of cluster %s: %w", cluster.Name, err)
	}
	for _, project := range projects {
		prtbs, err := b.prtbCache.List(project.GetProjectBackingNamespace(), labels.Everything())
		if err != nil {
			return 0, fmt.Errorf("listing role template bindings of project %s/%s: %w", project.Namespace, project.Name, err)
		}
		remaining += len(prtbs)
		for _, prtb := range prtbs {
			if prtb.DeletionTimestamp != nil {
				continue
			}
			if err := b.prtbs.Delete(prtb.Namespace, prtb.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return 0, fmt.Errorf("deleting project role template binding %s/%s: %w", prtb.Namespace, prtb.Name, err)
			}
		}
	}
	return remaining, nil
}
