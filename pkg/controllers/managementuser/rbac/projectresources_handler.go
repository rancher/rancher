package rbac

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rancher/rancher/pkg/api/steve/projectresources"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectResourcesSuffix = "-projectresources"
	enqueueAfterPeriod     = 10 * time.Second
)

// errNotReady is used when the APIResourceWatcher has not performed a successful discovery yet.
var errNotReady = errors.New("APIs have not been synced yet")

// getProjectResourcesRules accepts a slice of rules and returns a matching set of applicable rules for the resources.project.cattle.io API.
// The returned list will only include rules that are 1) not for resource names, 2) not for non-resource URLs, 3) use the "list" or "*" verbs, and are namespaced.
// It returns early if none of the rules match the first 3 conditions, which means the role binding can be skipped.
// It uses the APIResourceWatcher to check if a resource is namespaced, because only namespaced resources are stored in the watcher.
// If the APIResourceWatcher has no contents yet, it is probably just starting up so we return an error to indicate that the function should be retried.
func (m *manager) getProjectResourcesRules(rules []rbacv1.PolicyRule) ([]rbacv1.PolicyRule, error) {
	validRules := []rbacv1.PolicyRule{}
	// check that at least one of the rules probably needs to be mirrored before consulting the APIResourceWatcher,
	// most likely we can return early success and the calling controller can move on from the object.
	for _, rule := range rules {
		if len(rule.ResourceNames) != 0 {
			continue
		}
		if len(rule.NonResourceURLs) != 0 {
			continue
		}
		for _, verb := range rule.Verbs {
			if verb == "*" || verb == "list" {
				validRules = append(validRules, rule)
				break
			}
		}
	}
	if len(validRules) == 0 {
		return []rbacv1.PolicyRule{}, nil
	}
	apis := m.apis.List()
	if len(apis) == 0 {
		return []rbacv1.PolicyRule{}, errNotReady
	}
	projectRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{projectresources.Group},
			Verbs:     []string{"list"},
		},
	}

	for _, rule := range validRules {
		for _, apiGroup := range rule.APIGroups {
			for _, resource := range rule.Resources {
				_, ok := m.apis.Get(resource, apiGroup)
				if ok {
					if apiGroup != "" {
						resource = apiGroup + "." + resource
					}
					projectRules[0].Resources = append(projectRules[0].Resources, resource)
					continue
				}
				if resource == "*" {
					if apiGroup == "*" {
						projectRules = []rbacv1.PolicyRule{
							{
								APIGroups: []string{projectresources.Group},
								Verbs:     []string{"list"},
								Resources: []string{"*"},
							},
						}
						return projectRules, nil
					}
					for _, api := range apis {
						if api.Group == apiGroup {
							projectRules[0].Resources = append(projectRules[0].Resources, api.Name)
						}
					}
					sort.Strings(projectRules[0].Resources)
					continue
				}
				if apiGroup == "*" {
					for _, api := range apis {
						if api.Group+"."+resource == api.Name {
							projectRules[0].Resources = append(projectRules[0].Resources, api.Name)
						}
					}
					sort.Strings(projectRules[0].Resources)
				}
			}
		}
	}
	if len(projectRules[0].Resources) == 0 {
		return []rbacv1.PolicyRule{}, nil
	}
	return projectRules, nil
}

// ensureProjectResourcesRoles mirrors clusterroles for the resources.project.cattle.io API or cleans up clusterroles if they are not needed.
// It expects the cluserrole on which it is based to exist already, so ensureClusterRoles must be called first.
func (m *manager) ensureProjectResourcesRoles(rt *v3.RoleTemplate) error {
	clusterRole, err := m.crLister.Get("", rt.Name)
	if err != nil {
		return fmt.Errorf("couldn't get parent clusterrole %s for project resource role: %w", rt.Name, err)
	}
	projectResourcesRoleName := rt.Name + projectResourcesSuffix
	projectResourcesClusterRole, err := m.crLister.Get("", projectResourcesRoleName)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("couldn't ensure project role: %w", err)
	}

	rules := clusterRole.Rules
	projectResourcesRT := rt.DeepCopy()
	projectResourcesRT.Rules, err = m.getProjectResourcesRules(rules)
	if err != nil {
		return fmt.Errorf("couldn't ensure project role: %w", err)
	}
	if projectResourcesClusterRole == nil && len(projectResourcesRT.Rules) == 0 {
		return nil
	}

	if projectResourcesClusterRole != nil {
		if len(projectResourcesRT.Rules) == 0 {
			err := m.clusterRoles.Delete(projectResourcesRoleName, &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return fmt.Errorf("couldn't clean up project role: %w", err)
			}
			return nil
		}
		err := m.compareAndUpdateClusterRole(projectResourcesClusterRole, projectResourcesRT)
		if err == nil {
			return nil
		}
		if apierrors.IsConflict(err) {
			// get object from etcd and retry
			projectResourcesClusterRole, err := m.clusterRoles.Get(projectResourcesRoleName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error getting clusterRole %s: %w", projectResourcesRoleName, err)
			}
			return m.compareAndUpdateClusterRole(projectResourcesClusterRole, projectResourcesRT)
		}
		return fmt.Errorf("couldn't update clusterRole %s: %w", projectResourcesRoleName, err)
	}

	err = m.createClusterRole(projectResourcesRoleName, projectResourcesRT, map[string]string{projectresources.AuthzAnnotation: rt.Name})
	if err != nil {
		return fmt.Errorf("couldn't create clusterRole %s: %w", projectResourcesRoleName, err)
	}
	return nil
}

// ensureProjectResourcesRoleBindings creates rolebindings in a project namespace to the projectresources clusterroles which are mirrors of the given roles.
func (m *manager) ensureProjectResourcesRoleBindings(ns string, roles map[string]*v3.RoleTemplate, binding *v3.ProjectRoleTemplateBinding) error {
	projectRoles := make(map[string]*v3.RoleTemplate)
	for name := range roles {
		projectRoles[name+projectResourcesSuffix] = nil
	}

	annotations := map[string]string{
		projectresources.AuthzAnnotation: pkgrbac.GetRTBLabel(binding.ObjectMeta),
	}

	// The rolebinding will have an rtb-owner-updated label with the PRTB's ID,
	// so the PRTB handler will clean it up when the PRTB is removed.
	return m.ensureProjectRoleBindings(ns, projectRoles, binding, annotations)
}

// ensureProjectResourcesClusterRoleBindings creates clusterrolebindings to the projectresources clusterroles which are mirrors of the given roles.
func (m *manager) ensureProjectResourcesClusterRoleBindings(roles map[string]*v3.RoleTemplate, binding *v3.ClusterRoleTemplateBinding) error {
	projectRoles := make(map[string]*v3.RoleTemplate)
	owners := make(map[string]metav1.OwnerReference)
	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		return fmt.Errorf("couldn't get subject from binding %s: %w", binding.Name, err)
	}
	for name := range roles {
		projectRoles[name+projectResourcesSuffix] = nil
		ownerName := pkgrbac.NameForClusterRoleBinding(rbacv1.RoleRef{Kind: "ClusterRole", Name: name}, subject)
		owner, err := m.crbLister.Get("", ownerName)
		if err != nil {
			return fmt.Errorf("couldn't get clusterrolebinding %s: %w", ownerName, err)
		}
		ownerRef := metav1.OwnerReference{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRoleBinding",
			UID:        owner.UID,
			Name:       owner.Name,
		}
		owners[name+projectResourcesSuffix] = ownerRef
	}

	annotations := map[string]string{
		projectresources.AuthzAnnotation: pkgrbac.GetRTBLabel(binding.ObjectMeta),
	}

	binding = binding.DeepCopy()
	// Rename the binding copy.
	// This ensures the clusterrolebinding will have a distinct rtb-owner-updated label, so that manager.ensureBindings does not clean it up.
	// The owner reference will ensure that it does get cleaned up when its parent is removed.
	binding.Name = binding.Name + projectResourcesSuffix

	return m.ensureClusterBindings(projectRoles, binding, annotations, owners)
}
