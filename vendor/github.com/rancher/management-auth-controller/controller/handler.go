package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectResource = "projects"
	clusterResource = "clusters"
)

func Register(ctx context.Context, management *config.ManagementContext) {
	mgr := &manager{
		mgmt:      management,
		ctx:       ctx,
		crbLister: management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:  management.RBAC.ClusterRoles("").Controller().Lister(),
	}
	prtbLifecycle := &prtbLifecycle{
		mgr:           mgr,
		projectLister: management.Management.Projects("").Controller().Lister(),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	crtbLifecycle := &crtbLifecycle{
		mgr:           mgr,
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}

	management.Management.ProjectRoleTemplateBindings("").AddLifecycle("management-auth-prtb-controller", prtbLifecycle)
	management.Management.ClusterRoleTemplateBindings("").AddLifecycle("management-auth-crtb-controller", crtbLifecycle)
}

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
	// Don't need to delete the created ClusterRoleBinding because owner reference will take care of that
	return nil, nil
}

func (p *prtbLifecycle) ensureBindings(binding *v3.ProjectRoleTemplateBinding) error {
	projectName := binding.ProjectName
	proj, err := p.projectLister.Get("", projectName)
	if err != nil {
		return err
	}
	if proj == nil {
		return errors.Errorf("cannot create binding because project %v was not found", projectName)
	}

	clusterName := proj.Spec.ClusterName
	cluster, err := p.clusterLister.Get("", clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("cannot create binding because cluster %v was not found", clusterName)
	}

	projectRoleName := strings.ToLower(fmt.Sprintf("%v-projectmembers", projectName))
	clusterRoleName := strings.ToLower(fmt.Sprintf("%v-clustermembers", clusterName))

	if err := p.mgr.ensureBinding(projectRoleName, projectResource, projectName, binding.Subject, binding.ObjectMeta,
		binding.TypeMeta, proj.ObjectMeta, proj.TypeMeta); err != nil {
		return err
	}
	return p.mgr.ensureBinding(clusterRoleName, clusterResource, clusterName, binding.Subject, binding.ObjectMeta,
		binding.TypeMeta, cluster.ObjectMeta, cluster.TypeMeta)
}

type crtbLifecycle struct {
	mgr           *manager
	clusterLister v3.ClusterLister
}

func (c *crtbLifecycle) Create(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	err := c.ensureBindings(obj)
	return obj, err
}

func (c *crtbLifecycle) Updated(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	err := c.ensureBindings(obj)
	return nil, err
}

func (c *crtbLifecycle) Remove(obj *v3.ClusterRoleTemplateBinding) (*v3.ClusterRoleTemplateBinding, error) {
	// Don't need to delete the created ClusterRoleBinding because owner reference will take care of that
	return nil, nil
}

func (c *crtbLifecycle) ensureBindings(binding *v3.ClusterRoleTemplateBinding) error {
	clusterName := binding.ClusterName
	cluster, err := c.clusterLister.Get("", clusterName)
	if err != nil {
		return err
	}
	if cluster == nil {
		return errors.Errorf("cannot create binding because cluster %v was not found", clusterName)
	}

	clusterRoleName := strings.ToLower(fmt.Sprintf("%v-clustermembers", clusterName))

	return c.mgr.ensureBinding(clusterRoleName, clusterResource, clusterName, binding.Subject, binding.ObjectMeta,
		binding.TypeMeta, cluster.ObjectMeta, cluster.TypeMeta)
}

type manager struct {
	ctx       context.Context
	crLister  typesrbacv1.ClusterRoleLister
	crbLister typesrbacv1.ClusterRoleBindingLister
	mgmt      *config.ManagementContext
}

func (m *manager) ensureRole(roleName, resource, resourceName string, ownerMeta metav1.ObjectMeta, ownerTypeMeta metav1.TypeMeta) error {
	roleCli := m.mgmt.RBAC.ClusterRoles("")
	if cr, _ := m.crLister.Get("", roleName); cr == nil {
		_, err := roleCli.Create(&v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: ownerTypeMeta.APIVersion,
						Kind:       ownerTypeMeta.Kind,
						Name:       ownerMeta.Name,
						UID:        ownerMeta.UID,
					},
				},
			},
			Rules: []v1.PolicyRule{
				{
					APIGroups:     []string{"management.cattle.io"},
					Resources:     []string{resource},
					ResourceNames: []string{resourceName},
					Verbs:         []string{"get"},
				},
			},
		})
		return err
	}
	return nil
}

func (m *manager) ensureBinding(roleName, resource, resourceName string, subject v1.Subject, bindingOwnerMeta metav1.ObjectMeta,
	bindingOwnerTypeMeta metav1.TypeMeta, roleOwnerMeta metav1.ObjectMeta, roleOwnerTypeMeta metav1.TypeMeta) error {
	if err := m.ensureRole(roleName, resource, resourceName, roleOwnerMeta, roleOwnerTypeMeta); err != nil {
		return err
	}
	name := strings.ToLower(fmt.Sprintf("%v-%v-%v", roleName, subject.Kind, subject.Name))
	crb, _ := m.crbLister.Get("", name)
	if crb == nil {
		_, err := m.mgmt.RBAC.ClusterRoleBindings("").Create(&v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: bindingOwnerTypeMeta.APIVersion,
						Kind:       bindingOwnerTypeMeta.Kind,
						Name:       bindingOwnerMeta.Name,
						UID:        bindingOwnerMeta.UID,
					},
				},
			},
			Subjects: []v1.Subject{subject},
			RoleRef: v1.RoleRef{
				Kind: "ClusterRole",
				Name: roleName,
			},
		})
		return err
	}
	found := false
	for _, o := range crb.OwnerReferences {
		if bindingOwnerMeta.UID == o.UID && bindingOwnerMeta.Name == o.Name {
			found = true
			break
		}
	}
	if !found {
		crb = crb.DeepCopy()
		crb.OwnerReferences = append(crb.OwnerReferences, metav1.OwnerReference{
			APIVersion: bindingOwnerTypeMeta.APIVersion,
			Kind:       bindingOwnerTypeMeta.Kind,
			Name:       bindingOwnerMeta.Name,
			UID:        bindingOwnerMeta.UID,
		})
		_, err := m.mgmt.RBAC.ClusterRoleBindings("").Update(crb)
		if err != nil {
			return err
		}
	}
	return nil
}
