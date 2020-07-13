package auth

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	v3 "github.com/rancher/rancher/pkg/types/apis/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	clusterResource             = "clusters"
	membershipBindingOwner      = "memberhsip-binding-owner"
	membershipBindingOwnerIndex = "auth.management.cattle.io/membership-binding-owner"
	crtbInProjectBindingOwner   = "crtb-in-project-binding-owner"
	prtbInClusterBindingOwner   = "prtb-in-cluster-binding-owner"
	rbByOwnerIndex              = "auth.management.cattle.io/rb-by-owner"
	rbByRoleAndSubjectIndex     = "auth.management.cattle.io/crb-by-role-and-subject"
	ctrbMGMTController          = "mgmt-auth-crtb-controller"
)

var clusterManagmentPlaneResources = map[string]string{
	"clusterscans":                "management.cattle.io",
	"catalogtemplates":            "management.cattle.io",
	"catalogtemplateversions":     "management.cattle.io",
	"clusteralertrules":           "management.cattle.io",
	"clusteralertgroups":          "management.cattle.io",
	"clustercatalogs":             "management.cattle.io",
	"clusterloggings":             "management.cattle.io",
	"clustermonitorgraphs":        "management.cattle.io",
	"clusterregistrationtokens":   "management.cattle.io",
	"clusterroletemplatebindings": "management.cattle.io",
	"etcdbackups":                 "management.cattle.io",
	"nodes":                       "management.cattle.io",
	"nodepools":                   "management.cattle.io",
	"notifiers":                   "management.cattle.io",
	"podsecuritypolicytemplateprojectbindings": "management.cattle.io",
	"projects": "management.cattle.io",
}

type crtbLifecycle struct {
	mgr           *manager
	clusterLister v3.ClusterLister
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	obj, err := c.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = c.reconcileBindings(obj)

	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	obj, err := c.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = c.reconcileBindings(obj)
	return obj, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (runtime.Object, error) {
	if err := c.mgr.reconcileClusterMembershipBindingForDelete("", string(obj.UID)); err != nil {
		return nil, err
	}
	err := c.removeMGMTClusterScopedPrivilegesInProjectNamespace(obj)
	return nil, err
}

func (c *crtbLifecycle) reconcileSubject(binding *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
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

	if binding.UserPrincipalName == "" && binding.UserName != "" {
		u, err := c.mgr.userLister.Get("", binding.UserName)
		if err != nil {
			return binding, err
		}
		for _, p := range u.PrincipalIDs {
			if strings.HasSuffix(p, binding.UserName) {
				binding.UserPrincipalName = p
				break
			}
		}
		return binding, nil
	}

	return nil, errors.Errorf("Binding %v has no subject", binding.Name)
}

// When a CRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC
// Specifically:
// - ensure the subject can see the cluster in the mgmt API
// - if the subject was granted owner permissions for the clsuter, ensure they can create/update/delete the cluster
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
func (c *crtbLifecycle) reconcileBindings(binding *v3.ClusterRoleTemplateBinding) error {
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
	// if roletemplate is not builtin, check if it's inherited/cloned
	isOwnerRole, err := c.mgr.checkReferencedRoles(binding.RoleTemplateName)
	if err != nil {
		return err
	}
	var clusterRoleName string
	if isOwnerRole {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clusterowner", clusterName))
	} else {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
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
		if err := c.mgr.grantManagementClusterScopedPrivilegesInProjectNamespace(binding.RoleTemplateName, p.Name, projectManagmentPlaneResources, subject, binding); err != nil {
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
			logrus.Infof("[%v] Deleting rolebinding %v in namespace %v for crtb %v", ctrbMGMTController, rb.Name, p.Name, binding.Name)
			if err := c.mgr.mgmt.RBAC.RoleBindings(p.Name).Delete(rb.Name, &v1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}
