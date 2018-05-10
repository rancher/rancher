package auth

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

const (
	projectResource = "projects"
)

var projectManagmentPlaneResources = []string{"projectroletemplatebindings", "apps", "secrets", "pipelines", "pipelineexecutions", "pipelineexecutionlogs", "projectloggings", "projectalerts"}
var clusterManagmentPlaneResourcesForProject = []string{"notifiers"}

type prtbLifecycle struct {
	mgr           *manager
	projectLister v3.ProjectLister
	clusterLister v3.ClusterLister
}

func (p *prtbLifecycle) Create(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := p.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = p.reconcileBindings(obj)
	return obj, err
}

func (p *prtbLifecycle) Updated(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	obj, err := p.reconcileSubject(obj)
	if err != nil {
		return nil, err
	}
	err = p.reconcileBindings(obj)
	return obj, err
}

func (p *prtbLifecycle) Remove(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
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
	return nil, err
}

func (p *prtbLifecycle) reconcileSubject(binding *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	if binding.UserName != "" || binding.GroupName != "" || binding.GroupPrincipalName != "" {
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

	return nil, errors.Errorf("Binding %v has no subject", binding.Name)
}

// When a PRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC.
// Specifically:
// - ensure the subject can see the project and its parent cluster in the mgmt API
// - if the subject was granted owner permissions for the project, ensure they can create/update/delete the project
// - if the subject was granted privileges to mgmt plane resources that are scoped to the project, enforce those rules in the project's mgmt plane namespace
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
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
	isOwnerRole := binding.RoleTemplateName == "project-owner"
	var projectRoleName string
	if isOwnerRole {
		projectRoleName = strings.ToLower(fmt.Sprintf("%v-projectowner", projectName))
	} else {
		projectRoleName = strings.ToLower(fmt.Sprintf("%v-projectmember", projectName))
	}

	subject, err := buildSubjectFromRTB(binding)
	if err != nil {
		return err
	}
	if err := p.mgr.ensureProjectMembershipBinding(projectRoleName, string(binding.UID), clusterName, proj, isOwnerRole, subject); err != nil {
		return err
	}
	if err := p.mgr.ensureClusterMembershipBinding(roleName, string(binding.UID), cluster, false, subject); err != nil {
		return err
	}

	if err := p.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, projectManagmentPlaneResources, subject, binding, binding.Namespace); err != nil {
		return err
	}

	return p.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, clusterManagmentPlaneResourcesForProject, subject, binding, clusterName)
}
