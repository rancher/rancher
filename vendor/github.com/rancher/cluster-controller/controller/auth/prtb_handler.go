package auth

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	"k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	projectResource = "projects"
)

var projectManagmentPlanResources = []string{"projectroletemplatebindings", "stacks"}

type prtbLifecycle struct {
	mgr           *manager
	projectLister v3.ProjectLister
	clusterLister v3.ClusterLister
}

func (p *prtbLifecycle) Create(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	err := p.ensureBindings(obj)
	return obj, err
}

func (p *prtbLifecycle) Updated(obj *v3.ProjectRoleTemplateBinding) (*v3.ProjectRoleTemplateBinding, error) {
	err := p.ensureBindings(obj)
	return nil, err
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

// When a PRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC.
// Specifically:
// - ensure the subject can see the project and its parent cluster in the mgmt API
// - if the subject was granted owner permissions for the project, ensure they can create/update/delete the project
// - if the subject was granted privileges to mgmt plane resources that are scoped to the project, enforce those rules in the project's mgmt plane namespace
func (p *prtbLifecycle) ensureBindings(binding *v3.ProjectRoleTemplateBinding) error {
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

	subject := buildSubjectFromPRTB(binding)
	if err := p.mgr.ensureProjectMembershipBinding(projectRoleName, string(binding.UID), clusterName, proj, isOwnerRole, subject); err != nil {
		return err
	}
	if err := p.mgr.ensureClusterMembershipBinding(roleName, clusterName, string(binding.UID), false, subject); err != nil {
		return err
	}

	return p.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, projectManagmentPlanResources, subject, binding)
}

// When a PRTB is created that gives a subject some permissions in a project or cluster, we need to create a "membership" binding
// that gives the subject access to the the project/cluster custom resource itself
func (m *manager) ensureProjectMembershipBinding(roleName, rtbUID, namespace string, project *v3.Project, makeOwner bool, subject v1.Subject) error {
	if err := m.createProjectMembershipRole(roleName, namespace, project, makeOwner); err != nil {
		return err
	}

	name := strings.ToLower(fmt.Sprintf("%v-%v", roleName, subject.Name))
	set := labels.Set(map[string]string{rtbUID: owner})
	rbs, err := m.rbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}
	var rb *v1.RoleBinding
	for _, iRB := range rbs {
		if iRB.Name == name {
			rb = iRB
			continue
		}
		if err := m.reconcileProjectMembershipBindingForDelete(namespace, roleName, rtbUID); err != nil {
			return err
		}
	}

	if rb != nil {
		return nil
	}

	rb, _ = m.rbLister.Get(namespace, name)
	if rb == nil {
		_, err := m.mgmt.RBAC.RoleBindings(namespace).Create(&v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					rtbUID: owner,
				},
			},
			Subjects: []v1.Subject{subject},
			RoleRef: v1.RoleRef{
				Kind: "Role",
				Name: roleName,
			},
		})
		return err
	}

	for owner := range rb.Labels {
		if rtbUID == owner {
			return nil
		}
	}

	rb = rb.DeepCopy()
	if rb.Labels == nil {
		rb.Labels = map[string]string{}
	}
	rb.Labels[rtbUID] = owner
	_, err = m.mgmt.RBAC.RoleBindings(namespace).Update(rb)
	return err
}

// Creates a role that lets the bound subject see (if they are an ordinary member) the project in the mgmt api
// (or CRUD the project if they are an owner)
func (m *manager) createProjectMembershipRole(roleName, namespace string, project *v3.Project, makeOwner bool) error {
	roleCli := m.mgmt.RBAC.Roles(namespace)
	if cr, _ := m.rLister.Get(namespace, roleName); cr == nil {
		rules := []v1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{projectResource},
				ResourceNames: []string{project.Name},
				Verbs:         []string{"get"},
			},
		}
		if makeOwner {
			rules[0].Verbs = []string{"*"}
		} else {
			rules[0].Verbs = []string{"get"}
		}
		_, err := roleCli.Create(&v1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: project.APIVersion,
						Kind:       project.Kind,
						Name:       project.Name,
						UID:        project.UID,
					},
				},
			},
			Rules: rules,
		})
		return err
	}
	return nil
}

// The PRTB has been deleted, either delete or update the project membership binding so that the subject
// is removed from the project if they should be
func (m *manager) reconcileProjectMembershipBindingForDelete(namespace, roleToKeep, rtbUID string) error {
	set := labels.Set(map[string]string{rtbUID: owner})
	rbs, err := m.rbLister.List(namespace, set.AsSelector())
	if err != nil {
		return err
	}

	for _, rb := range rbs {
		if rb.RoleRef.Name == roleToKeep {
			continue
		}

		rb = rb.DeepCopy()
		for k, v := range rb.Labels {
			if k == rtbUID && v == owner {
				delete(rb.Labels, k)
			}
		}

		if len(rb.Labels) == 0 {
			if err := m.mgmt.RBAC.RoleBindings(namespace).Delete(rb.Name, &metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
		} else {
			if _, err := m.mgmt.RBAC.RoleBindings(namespace).Update(rb); err != nil {
				return err
			}
		}
	}

	return nil
}

func buildSubjectFromPRTB(binding *v3.ProjectRoleTemplateBinding) v1.Subject {
	return v1.Subject{
		Kind: "User",
		Name: binding.UserName,
	}
}
