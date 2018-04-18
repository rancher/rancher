package auth

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	clusterResource           = "clusters"
	membershipBindingOwner    = "memberhsip-binding-owner"
	crtbInProjectBindingOwner = "crtb-in-project-binding-owner"
	rbByOwnerIndex            = "auth.management.cattle.io/rb-by-owner"
	rbByRoleAndSubjectIndex   = "auth.management.cattle.io/crb-by-role-and-subject"
)

var clusterManagmentPlaneResources = []string{"clusterroletemplatebindings", "nodes", "nodepools", "clusterevents", "projects", "clusterregistrationtokens", "clusterpipelines", "clusterloggings", "notifiers", "clusteralerts"}

type crtbLifecycle struct {
	mgr           *manager
	clusterLister v3.ClusterLister
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	obj, err := c.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = c.reconcilBindings(obj)
	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	obj, err := c.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = c.reconcilBindings(obj)
	return obj, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if err := c.mgr.reconcileClusterMembershipBindingForDelete("", string(obj.UID)); err != nil {
		return nil, err
	}
	err := c.removeMGMTClusterScopedPrivilegesInProjectNamespace(obj)
	return nil, err
}

func (c *crtbLifecycle) reconcileSubject(binding *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if binding.UserName != "" || binding.GroupName != "" || binding.GroupPrincipalName != "" {
		return binding, nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := c.mgr.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			return binding, err
		}

		binding.UserName = user.Name
		return binding, nil
	}

	return nil, errors.Errorf("Binding %v has no subject", binding.Name)
}

// When a CRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC
// Specifically:
// - ensure the subject can see the cluster in the mgmt API
// - if the subject was granted owner permissions for the clsuter, ensure they can create/update/delete the cluster
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
func (c *crtbLifecycle) reconcilBindings(binding *v3.ClusterRoleTemplateBinding) error {
	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		return nil
	}

	clusterName := binding.ClusterName
	cluster, err := c.clusterLister.Get("", clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("cannot create binding because cluster %v was not found", clusterName)
	}

	isOwnerRole := binding.RoleTemplateName == "cluster-owner"
	var clusterRoleName string
	if isOwnerRole {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clusterowner", clusterName))
	} else {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	}

	subject, err := buildSubjectFromRTB(binding)
	if err != nil {
		return err
	}
	if err := c.mgr.ensureClusterMembershipBinding(clusterRoleName, string(binding.UID), cluster, isOwnerRole, subject); err != nil {
		return err
	}

	err = c.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, clusterManagmentPlaneResources, subject, binding)
	if err != nil {
		return err
	}

	projects, err := c.mgr.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		return err
	}
	for _, p := range projects {
		if err := c.mgr.grantManagementClusterScopedPrivilegesInProjectNamespace(binding.RoleTemplateName, p.Name, projectManagmentPlanResources, subject, binding); err != nil {
			return err
		}
	}
	return nil
}

func (c *crtbLifecycle) removeMGMTClusterScopedPrivilegesInProjectNamespace(binding *v3.ClusterRoleTemplateBinding) error {
	projects, err := c.mgr.projectLister.List(binding.Namespace, labels.Everything())
	if err != nil {
		return err
	}
	for _, p := range projects {
		set := labels.Set(map[string]string{string(binding.UID): crtbInProjectBindingOwner})
		rbs, err := c.mgr.rbLister.List(p.Name, set.AsSelector())
		if err != nil {
			return err
		}
		for _, rb := range rbs {
			if err := c.mgr.mgmt.RBAC.RoleBindings(p.Name).Delete(rb.Name, &v1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}
