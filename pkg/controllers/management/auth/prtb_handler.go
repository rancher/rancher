package auth

import (
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/sirupsen/logrus"
)

const (
	projectResource    = "projects"
	ptrbMGMTController = "mgmt-auth-prtb-controller"
)

var projectManagmentPlaneResources = map[string]string{
	"apps":                        "project.cattle.io",
	"apprevisions":                "project.cattle.io",
	"catalogtemplates":            "management.cattle.io",
	"catalogtemplateversions":     "management.cattle.io",
	"pipelines":                   "project.cattle.io",
	"pipelineexecutions":          "project.cattle.io",
	"pipelinesettings":            "project.cattle.io",
	"sourcecodeproviderconfigs":   "project.cattle.io",
	"projectloggings":             "management.cattle.io",
	"projectalertrules":           "management.cattle.io",
	"projectalertgroups":          "management.cattle.io",
	"projectcatalogs":             "management.cattle.io",
	"projectmonitorgraphs":        "management.cattle.io",
	"projectroletemplatebindings": "management.cattle.io",
	"secrets":                     "",
}
var prtbClusterManagmentPlaneResources = map[string]string{
	"notifiers":               "management.cattle.io",
	"clustercatalogs":         "management.cattle.io",
	"catalogtemplates":        "management.cattle.io",
	"catalogtemplateversions": "management.cattle.io",
}

type prtbLifecycle struct {
	mgr           *manager
	projectLister v3.ProjectLister
	clusterLister v3.ClusterLister
}

func (p *prtbLifecycle) Create(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	if obj.ServiceAccount != "" {
		return obj, nil
	}
	obj, err := p.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = p.reconcileBindings(obj)
	return obj, err
}

func (p *prtbLifecycle) Updated(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	if obj.ServiceAccount != "" {
		return obj, nil
	}
	obj, err := p.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = p.reconcileBindings(obj)
	return obj, err
}

func (p *prtbLifecycle) Remove(obj *v3.ProjectRoleTemplateBinding) (runtime.Object, error) {
	parts := strings.SplitN(obj.ProjectName, ":", 2)
	if len(parts) < 2 {
		return nil, errors.Errorf("cannot determine project and cluster from %v", obj.ProjectName)
	}
	clusterName := parts[0]
	err := p.mgr.reconcileProjectMembershipBindingForDelete(clusterName, "", string(obj.UID))
	if err != nil {
		return nil, err
	}
	err = p.mgr.reconcileClusterMembershipBindingForDelete("", string(obj.UID))
	if err != nil {
		return nil, err
	}

	err = p.removeMGMTProjectScopedPrivilegesInClusterNamespace(obj, clusterName)
	return nil, err
}

func (p *prtbLifecycle) reconcileSubject(binding *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if binding.GroupName != "" || binding.GroupPrincipalName != "" || (binding.UserPrincipalName != "" && binding.UserName != "") {
		return binding, nil
	}

	if binding.UserPrincipalName != "" && binding.UserName == "" {
		displayName := binding.Annotations["auth.cattle.io/principal-display-name"]
		user, err := p.mgr.userMGR.EnsureUser(binding.UserPrincipalName, displayName)
		if err != nil {
			return binding, err
		}

		binding.UserName = user.Name
		return binding, nil
	}

	if binding.UserPrincipalName == "" && binding.UserName != "" {
		u, err := p.mgr.userLister.Get("", binding.UserName)
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

// When a PRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC.
// Specifically:
// - ensure the subject can see the project and its parent cluster in the mgmt API
// - if the subject was granted owner permissions for the project, ensure they can create/update/delete the project
// - if the subject was granted privileges to mgmt plane resources that are scoped to the project, enforce those rules in the project's mgmt plane namespace
func (p *prtbLifecycle) reconcileBindings(binding *v3.ProjectRoleTemplateBinding) error {
	if binding.UserName == "" && binding.GroupPrincipalName == "" && binding.GroupName == "" {
		return nil
	}

	parts := strings.SplitN(binding.ProjectName, ":", 2)
	if len(parts) < 2 {
		return errors.Errorf("cannot determine project and cluster from %v", binding.ProjectName)
	}

	clusterName := parts[0]
	projectName := parts[1]
	proj, err := p.projectLister.Get(clusterName, projectName)
	if err != nil {
		return err
	}
	if proj == nil {
		return errors.Errorf("cannot create binding because project %v was not found", projectName)
	}

	cluster, err := p.clusterLister.Get("", clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("cannot create binding because cluster %v was not found", clusterName)
	}

	roleName := strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	// if roletemplate is not builtin, check if it's inherited/cloned
	isOwnerRole, err := p.mgr.checkReferencedRoles(binding.RoleTemplateName)
	if err != nil {
		return err
	}
	var projectRoleName string
	if isOwnerRole {
		projectRoleName = strings.ToLower(fmt.Sprintf("%v-projectowner", projectName))
	} else {
		projectRoleName = strings.ToLower(fmt.Sprintf("%v-projectmember", projectName))
	}

	subject, err := pkgrbac.BuildSubjectFromRTB(binding)
	if err != nil {
		return err
	}
	if err := p.mgr.ensureProjectMembershipBinding(projectRoleName, string(binding.UID), clusterName, proj, isOwnerRole, subject); err != nil {
		return err
	}
	if err := p.mgr.ensureClusterMembershipBinding(roleName, string(binding.UID), cluster, false, subject); err != nil {
		return err
	}
	if err := p.mgr.grantManagementProjectScopedPrivilegesInClusterNamespace(binding.RoleTemplateName, proj.Namespace, prtbClusterManagmentPlaneResources, subject, binding); err != nil {
		return err
	}
	return p.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, projectManagmentPlaneResources, subject, binding)
}

// removeMGMTProjectScopedPrivilegesInClusterNamespace revokes access that project roles were granted to certain cluster scoped resources like
// catalogtemplates, when the prtb is deleted, by deleting the rolebinding created for this prtb in the cluster's namespace
func (p *prtbLifecycle) removeMGMTProjectScopedPrivilegesInClusterNamespace(binding *v3.ProjectRoleTemplateBinding, clusterName string) error {
	set := labels.Set(map[string]string{string(binding.UID): prtbInClusterBindingOwner})
	rbs, err := p.mgr.rbLister.List(clusterName, set.AsSelector())
	if err != nil {
		return err
	}
	for _, rb := range rbs {
		sub := rb.Subjects
		if sub[0].Name == binding.UserName {
			logrus.Infof("[%v] Deleting rolebinding %v in namespace %v for prtb %v", ptrbMGMTController, rb.Name, clusterName, binding.Name)
			if err := p.mgr.mgmt.RBAC.RoleBindings(clusterName).Delete(rb.Name, &v1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}
