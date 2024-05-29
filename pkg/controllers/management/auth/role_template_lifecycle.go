package auth

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/rancher/pkg/clustermanager"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/apply"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	roleTemplateLifecycleName = "mgmt-auth-roletemplate-lifecycle"
	prtbByRoleTemplateIndex   = "management.cattle.io/prtb-by-role-template"
	crtbByRoleTemplateIndex   = "management.cattle.io/crtb-by-role-template"
)

type roleTemplateLifecycle struct {
	prtbIndexer    cache.Indexer
	prtbClient     v3.ProjectRoleTemplateBindingInterface
	crtbIndexer    cache.Indexer
	crtbClient     v3.ClusterRoleTemplateBindingInterface
	clusters       v3.ClusterInterface
	roles          rbacv1.RoleInterface
	roleLister     rbacv1.RoleLister
	clusterManager *clustermanager.Manager
}

func newRoleTemplateLifecycle(management *config.ManagementContext, clusterManager *clustermanager.Manager) v3.RoleTemplateLifecycle {
	prtbInformer := management.Management.ProjectRoleTemplateBindings("").Controller().Informer()
	crtbInformer := management.Management.ClusterRoleTemplateBindings("").Controller().Informer()

	rtl := &roleTemplateLifecycle{
		prtbIndexer:    prtbInformer.GetIndexer(),
		prtbClient:     management.Management.ProjectRoleTemplateBindings(""),
		crtbIndexer:    crtbInformer.GetIndexer(),
		crtbClient:     management.Management.ClusterRoleTemplateBindings(""),
		clusters:       management.Management.Clusters(""),
		roles:          management.RBAC.Roles(""),
		roleLister:     management.RBAC.Roles("").Controller().Lister(),
		clusterManager: clusterManager,
	}
	return rtl
}

func (rtl *roleTemplateLifecycle) Create(obj *v3.RoleTemplate) (runtime.Object, error) {
	return rtl.enqueueRtbs(obj)
}

func (rtl *roleTemplateLifecycle) Updated(obj *v3.RoleTemplate) (runtime.Object, error) {
	return rtl.enqueueRtbs(obj)
}

// enqueueRtbs enqueues crtbs and prtbs associated to the role template.
func (rtl *roleTemplateLifecycle) enqueueRtbs(obj *v3.RoleTemplate) (runtime.Object, error) {
	if err := rtl.enqueuePrtbs(obj); err != nil {
		return nil, err
	}

	if err := rtl.enqueueCrtbs(obj); err != nil {
		return nil, err
	}

	return nil, nil
}

func (rtl *roleTemplateLifecycle) Remove(obj *v3.RoleTemplate) (runtime.Object, error) {
	clusters, err := rtl.clusters.List(metav1.ListOptions{})
	if err != nil {
		return obj, err
	}

	// Collect all the errors to delete as many user context cluster roles as possible
	var allErrors []error

	for _, cluster := range clusters.Items {
		userContext, err := rtl.clusterManager.UserContext(cluster.Name)
		if err != nil {
			// ClusterUnavailable error indicates the record can't talk to the downstream cluster
			if !clustermanager.IsClusterUnavailableErr(err) {
				allErrors = append(allErrors, err)
			}
			continue
		}

		b, err := userContext.RBAC.ClusterRoles("").Controller().Lister().Get("", obj.Name)
		if err != nil {
			// User context clusterRole doesn't exist
			if !apierrors.IsNotFound(err) {
				allErrors = append(allErrors, err)
			}
			continue
		}

		err = userContext.RBAC.ClusterRoles("").Delete(b.Name, &metav1.DeleteOptions{})
		if err != nil {
			// User context clusterRole doesn't exist
			if !apierrors.IsNotFound(err) {
				allErrors = append(allErrors, err)
			}
			continue
		}
	}

	if len(allErrors) > 0 {
		return obj, fmt.Errorf("errors deleting dowstream clusterRole: %v", allErrors)
	}

	return obj, rtl.removeAuthV2Roles(obj)
}

// removeAuthV2Roles finds any roles based off the owner annotation from the incoming roleTemplate.
// This is similar to an ownerReference but this is used across namespaces which ownerReferences does not support.
func (rtl *roleTemplateLifecycle) removeAuthV2Roles(roleTemplate *v3.RoleTemplate) error {
	// Get the selector for the dependent roles
	selector, err := apply.GetSelectorFromOwner("auth-prov-v2-roletemplate", roleTemplate)
	if err != nil {
		return err
	}

	roles, err := rtl.roleLister.List("", selector)
	if err != nil {
		return err
	}

	var returnErr error
	for _, role := range roles {
		err := rtl.roles.DeleteNamespaced(role.Namespace, role.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// Combine all errors so we try our best to delete everything in the first run
			returnErr = multierror.Append(returnErr, err)
		}
	}

	return returnErr
}

// enqueue any prtbs linked to this roleTemplate in order to re-sync them via reconcileBindings
func (rtl *roleTemplateLifecycle) enqueuePrtbs(updatedRT *v3.RoleTemplate) error {
	prtbs, err := rtl.prtbIndexer.ByIndex(prtbByRoleTemplateIndex, updatedRT.Name)
	if err != nil {
		return err
	}
	for _, x := range prtbs {
		if prtb, ok := x.(*v3.ProjectRoleTemplateBinding); ok {
			rtl.prtbClient.Controller().Enqueue(prtb.Namespace, prtb.Name)
		}
	}
	return nil
}

// enqueue any crtbs linked to this roleTemplate in order to re-sync them via reconcileBindings
func (rtl *roleTemplateLifecycle) enqueueCrtbs(updatedRT *v3.RoleTemplate) error {
	crtbs, err := rtl.crtbIndexer.ByIndex(crtbByRoleTemplateIndex, updatedRT.Name)
	if err != nil {
		return err
	}
	for _, x := range crtbs {
		if crtb, ok := x.(*v3.ClusterRoleTemplateBinding); ok {
			rtl.crtbClient.Controller().Enqueue(crtb.Namespace, crtb.Name)
		}
	}
	return nil
}

func prtbByRoleTemplate(obj interface{}) ([]string, error) {
	prtb, ok := obj.(*v3.ProjectRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{prtb.RoleTemplateName}, nil
}

func crtbByRoleTemplate(obj interface{}) ([]string, error) {
	crtb, ok := obj.(*v3.ClusterRoleTemplateBinding)
	if !ok {
		return []string{}, nil
	}
	return []string{crtb.RoleTemplateName}, nil
}
