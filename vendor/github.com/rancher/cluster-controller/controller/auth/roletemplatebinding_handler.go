package auth

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	projectResource = "projects"
	clusterResource = "clusters"
)

func newRTBLifecycles(management *config.ManagementContext) (*prtbLifecycle, *crtbLifecycle) {
	mgr := &manager{
		mgmt:      management,
		crbLister: management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:  management.RBAC.ClusterRoles("").Controller().Lister(),
		rLister:   management.RBAC.Roles("").Controller().Lister(),
		rbLister:  management.RBAC.RoleBindings("").Controller().Lister(),
		rtLister:  management.Management.RoleTemplates("").Controller().Lister(),
	}
	prtb := &prtbLifecycle{
		mgr:           mgr,
		projectLister: management.Management.Projects("").Controller().Lister(),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	crtb := &crtbLifecycle{
		mgr:           mgr,
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	return prtb, crtb
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

	clusterRoleName := strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	isOwnerRole := binding.RoleTemplateName == "project-owner"
	var projectRoleName string
	if isOwnerRole {
		projectRoleName = strings.ToLower(fmt.Sprintf("%v-projectowner", projectName))
	} else {
		projectRoleName = strings.ToLower(fmt.Sprintf("%v-projectmember", projectName))
	}

	if err := p.mgr.ensureMembershipBinding(projectRoleName, projectResource, projectName, isOwnerRole, binding.Subject, binding.ObjectMeta,
		binding.TypeMeta, proj.ObjectMeta, proj.TypeMeta); err != nil {
		return err
	}
	if err := p.mgr.ensureMembershipBinding(clusterRoleName, clusterResource, clusterName, false, binding.Subject, binding.ObjectMeta,
		binding.TypeMeta, cluster.ObjectMeta, cluster.TypeMeta); err != nil {
		return err
	}

	return p.mgr.ensureRTBBinding(binding.Namespace, binding.RoleTemplateName, "projectroletemplatebindings", binding.Subject, binding)
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

	isOwnerRole := binding.RoleTemplateName == "cluster-owner"
	var clusterRoleName string
	if isOwnerRole {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clusterowner", clusterName))
	} else {
		clusterRoleName = strings.ToLower(fmt.Sprintf("%v-clustermember", clusterName))
	}

	if err := c.mgr.ensureMembershipBinding(clusterRoleName, clusterResource, clusterName, isOwnerRole, binding.Subject, binding.ObjectMeta,
		binding.TypeMeta, cluster.ObjectMeta, cluster.TypeMeta); err != nil {
		return err
	}

	return c.mgr.ensureRTBBinding(binding.Namespace, binding.RoleTemplateName, "clusterroletemplatebindings", binding.Subject, binding)
}

type manager struct {
	crLister  typesrbacv1.ClusterRoleLister
	rLister   typesrbacv1.RoleLister
	rbLister  typesrbacv1.RoleBindingLister
	crbLister typesrbacv1.ClusterRoleBindingLister
	rtLister  v3.RoleTemplateLister
	mgmt      *config.ManagementContext
}

func (m *manager) ensureRole(roleName, resource, resourceName string, isOwnerRole bool, ownerMeta metav1.ObjectMeta, ownerTypeMeta metav1.TypeMeta) error {
	roleCli := m.mgmt.RBAC.ClusterRoles("")
	if cr, _ := m.crLister.Get("", roleName); cr == nil {
		rules := []v1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{resource},
				ResourceNames: []string{resourceName},
				Verbs:         []string{"get"},
			},
		}
		if isOwnerRole {
			rules[0].Verbs = []string{"*"}
		} else {
			rules[0].Verbs = []string{"get"}
		}
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
			Rules: rules,
		})
		return err
	}
	return nil
}

func (m *manager) ensureMembershipBinding(roleName, resource, resourceName string, isOwnerRole bool, subject v1.Subject, bindingOwnerMeta metav1.ObjectMeta,
	bindingOwnerTypeMeta metav1.TypeMeta, roleOwnerMeta metav1.ObjectMeta, roleOwnerTypeMeta metav1.TypeMeta) error {
	if err := m.ensureRole(roleName, resource, resourceName, isOwnerRole, roleOwnerMeta, roleOwnerTypeMeta); err != nil {
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

// If the RoleTemplate being bound to grants privileges to PRTB/CRTB, create a role and binding the the project/cluster namespace
// in the management plane
func (m *manager) ensureRTBBinding(namespace, roleTemplateName, resource string, subject v1.Subject, binding interface{}) error {
	metaAccessor, err := meta.Accessor(binding)
	if err != nil {
		return err
	}
	typeAccessor, err := meta.TypeAccessor(binding)
	if err != nil {
		return err
	}

	// gather roles that have PRTB/CRTB rules
	rt, err := m.rtLister.Get("", roleTemplateName)
	if err != nil {
		return err
	}
	allRoles := map[string]*v3.RoleTemplate{}
	if err := m.gatherRTBRoles(rt, allRoles); err != nil {
		return err
	}
	rtbRoles := map[string]*v3.RoleTemplate{}
	rtbVerbs := map[string]map[string]bool{}
	for _, role := range allRoles {
		verbs := map[string]bool{}
		rules, err := m.getRules(role)
		if err != nil {
			return err
		}
		for _, rule := range rules {
			if (slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, "*")) && len(rule.ResourceNames) == 0 && len(rule.Verbs) > 0 {
				rtbRoles[role.Name] = role
				for _, v := range rule.Verbs {
					verbs[v] = true
				}
			}
		}
		if len(verbs) > 0 {
			rtbVerbs[role.Name] = verbs
		}
	}

	// if no roles have CRTB/PRTB rules, nothing to do
	if len(rtbRoles) == 0 {
		return nil
	}

	// create or find namespaced roles
	if err := m.ensureRTBRoles(namespace, resource, rtbRoles, rtbVerbs); err != nil {
		return err
	}

	// creating binding for user for each role in namespace
	bindingCli := m.mgmt.RBAC.RoleBindings(namespace)
	for roleName := range rtbRoles {
		bindingName := metaAccessor.GetName()
		if b, _ := m.rbLister.Get(namespace, bindingName); b != nil {
			return nil
		}

		_, err := bindingCli.Create(&v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: bindingName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: typeAccessor.GetAPIVersion(),
						Kind:       typeAccessor.GetKind(),
						Name:       metaAccessor.GetName(),
						UID:        metaAccessor.GetUID(),
					},
				},
			},
			Subjects: []v1.Subject{subject},
			RoleRef: v1.RoleRef{
				Kind: "Role",
				Name: roleName,
			},
		})

		if err != nil {
			return errors.Wrapf(err, "couldn't ensure binding %v %v in %v", roleName, subject.Name, namespace)
		}
	}

	return nil
}

func (m *manager) getRules(role *v3.RoleTemplate) ([]v1.PolicyRule, error) {
	if !role.External {
		return role.Rules, nil
	}

	r, err := m.crLister.Get("", role.Name)
	if err != nil {
		return []v1.PolicyRule{}, err
	}

	return r.Rules, nil
}

func (m *manager) ensureRTBRoles(namespace, resource string, rts map[string]*v3.RoleTemplate, roleRTBVerbs map[string]map[string]bool) error {
	roleCli := m.mgmt.RBAC.Roles(namespace)
	for name, rt := range rts {
		wantedVerbs, ok := roleRTBVerbs[name]
		if !ok {
			return errors.Errorf("couldn't find verbs for %v", name)
		}

		if role, err := m.rLister.Get(namespace, rt.Name); err == nil && role != nil {
			currentVerbs := map[string]bool{}
			for _, rule := range role.Rules {
				if slice.ContainsString(rule.Resources, resource) {
					for _, v := range rule.Verbs {
						currentVerbs[v] = true
					}
				}
			}

			if !reflect.DeepEqual(currentVerbs, wantedVerbs) {
				role = role.DeepCopy()
				rules := buildRTBRules(resource, wantedVerbs)
				role.Rules = rules
				_, err := roleCli.Update(role)
				if err != nil {
					return errors.Wrapf(err, "couldn't update role %v", rt.Name)
				}
			}
			continue
		}

		rules := buildRTBRules(resource, wantedVerbs)
		_, err := roleCli.Create(&v1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name: rt.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: rt.TypeMeta.APIVersion,
						Kind:       rt.TypeMeta.Kind,
						Name:       rt.Name,
						UID:        rt.UID,
					},
				},
			},
			Rules: rules,
		})
		if err != nil {
			return errors.Wrapf(err, "couldn't create role %v", rt.Name)
		}
	}

	return nil
}

func (m *manager) gatherRTBRoles(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	roleTemplates[rt.Name] = rt

	for _, rtName := range rt.RoleTemplateNames {
		subRT, err := m.rtLister.Get("", rtName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get RoleTemplate %s", rtName)
		}
		if err := m.gatherRTBRoles(subRT, roleTemplates); err != nil {
			return errors.Wrapf(err, "couldn't gather RoleTemplate %s", rtName)
		}
	}

	return nil
}

func buildRTBRules(resource string, verbs map[string]bool) []v1.PolicyRule {
	var vs []string
	for v := range verbs {
		vs = append(vs, v)
	}
	return []v1.PolicyRule{
		{
			Resources: []string{resource},
			Verbs:     vs,
			APIGroups: []string{"*"},
		},
	}
}
