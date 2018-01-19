package auth

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/norman/types/slice"
	v12 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	clusterResource = "clusters"
	owner           = "owner"
)

var clusterManagmentPlanResources = []string{"clusterroletemplatebindings", "machines", "clusterevents", "projects", "clusterregistrationtokens"}

func newRTBLifecycles(management *config.ManagementContext) (*prtbLifecycle, *crtbLifecycle) {
	mgr := &manager{
		mgmt:      management,
		crbLister: management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:  management.RBAC.ClusterRoles("").Controller().Lister(),
		rLister:   management.RBAC.Roles("").Controller().Lister(),
		rbLister:  management.RBAC.RoleBindings("").Controller().Lister(),
		rtLister:  management.Management.RoleTemplates("").Controller().Lister(),
		nsLister:  management.Core.Namespaces("").Controller().Lister(),
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
	err := c.mgr.reconcileClusterMembershipBindingForDelete(string(obj.UID))
	return nil, err
}

// When a CRTB is created or updated, translate it into several k8s roles and bindings to actually enforce the RBAC
// Specifically:
// - ensure the subject can see the cluster in the mgmt API
// - if the subject was granted owner permissions for the clsuter, ensure they can create/update/delete the cluster
// - if the subject was granted privileges to mgmt plane resources that are scoped to the cluster, enforce those rules in the cluster's mgmt plane namespace
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

	if err := c.mgr.ensureClusterMembershipBinding(clusterRoleName, clusterName, string(binding.UID), isOwnerRole, binding.Subject); err != nil {
		return err
	}

	for _, resource := range clusterManagmentPlanResources {
		if err := c.mgr.grantManagementPlanePrivileges(binding.RoleTemplateName, resource, binding.Subject, binding); err != nil {
			return err
		}
	}

	return nil
}

type manager struct {
	crLister  typesrbacv1.ClusterRoleLister
	rLister   typesrbacv1.RoleLister
	rbLister  typesrbacv1.RoleBindingLister
	crbLister typesrbacv1.ClusterRoleBindingLister
	rtLister  v3.RoleTemplateLister
	nsLister  v12.NamespaceLister
	mgmt      *config.ManagementContext
}

// When a CRTB is created that gives a subject some permissions in a project or cluster, we need to create a "membership" binding
// that gives the subject access to the the cluster custom resource itself
func (m *manager) ensureClusterMembershipBinding(roleName, clusterName, rtbUID string, makeOwner bool, subject v1.Subject) error {
	if err := m.createClusterMembershipRole(roleName, clusterName, makeOwner); err != nil {
		return err
	}

	name := strings.ToLower(fmt.Sprintf("%v-%v", roleName, subject.Name))
	crb, _ := m.crbLister.Get("", name)
	if crb == nil {
		_, err := m.mgmt.RBAC.ClusterRoleBindings("").Create(&v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					rtbUID: owner,
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

	for owner := range crb.Labels {
		if rtbUID == owner {
			return nil
		}
	}

	crb = crb.DeepCopy()
	if crb.Labels == nil {
		crb.Labels = map[string]string{}
	}
	crb.Labels[rtbUID] = owner
	_, err := m.mgmt.RBAC.ClusterRoleBindings("").Update(crb)
	return err
}

// Creates a role that lets the bound subject see (if they are an ordinary member) the project or cluster in the mgmt api
// (or CRUD the project/cluster if they are an owner)
func (m *manager) createClusterMembershipRole(roleName, clusterName string, makeOwner bool) error {
	roleCli := m.mgmt.RBAC.ClusterRoles("")
	ns, err := m.nsLister.Get("", clusterName)
	if err != nil {
		return err
	}
	if cr, _ := m.crLister.Get("", roleName); cr == nil {
		rules := []v1.PolicyRule{
			{
				APIGroups:     []string{"management.cattle.io"},
				Resources:     []string{clusterResource},
				ResourceNames: []string{clusterName},
				Verbs:         []string{"get"},
			},
		}
		if makeOwner {
			rules[0].Verbs = []string{"*"}
		} else {
			rules[0].Verbs = []string{"get"}
		}
		_, err := roleCli.Create(&v1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: roleName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: ns.APIVersion,
						Kind:       ns.Kind,
						Name:       ns.Name,
						UID:        ns.UID,
					},
				},
			},
			Rules: rules,
		})
		return err
	}
	return nil
}

// The CRTB has been deleted, either delete or update the membership binding so that the subject
// is removed from the cluster if they should be
func (m *manager) reconcileClusterMembershipBindingForDelete(rtbUID string) error {
	set := labels.Set(map[string]string{rtbUID: owner})
	crbs, err := m.crbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}

	for _, crb := range crbs {
		crb = crb.DeepCopy()
		for k, v := range crb.Labels {
			if k == rtbUID && v == owner {
				delete(crb.Labels, k)
			}
		}

		if len(crb.Labels) == 0 {
			if err := m.mgmt.RBAC.ClusterRoleBindings("").Delete(crb.Name, &metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
		} else {
			if _, err := m.mgmt.RBAC.ClusterRoleBindings("").Update(crb); err != nil {
				return err
			}
		}
	}

	return nil
}

// Certain resources (projects, machines, prtbs, crtbs, clusterevents, etc) exist in the mangement plane but are scoped to clusters or
// projects. They need special RBAC handling because the need to be authorized just inside of the namespace that backs the project
// or cluster they belong to.
func (m *manager) grantManagementPlanePrivileges(roleTemplateName, resource string, subject v1.Subject, binding interface{}) error {
	bindingMeta, err := meta.Accessor(binding)
	if err != nil {
		return err
	}
	bindingTypeMeta, err := meta.TypeAccessor(binding)
	if err != nil {
		return err
	}
	namespace := bindingMeta.GetNamespace()

	// gather roles that have rules for mgmt plane resources
	rt, err := m.rtLister.Get("", roleTemplateName)
	if err != nil {
		return err
	}
	allRoles := map[string]*v3.RoleTemplate{}
	if err := m.gatherRoleTemplates(rt, allRoles); err != nil {
		return err
	}

	//de-dupe
	roles := map[string]*v3.RoleTemplate{}
	for _, role := range allRoles {
		roles[role.Name] = role
	}

	bindingCli := m.mgmt.RBAC.RoleBindings(namespace)
	for _, role := range roles {
		verbs := m.checkForManagementPlaneRules(role, resource)
		if len(verbs) > 0 {
			if err := m.reconcileManagementPlaneRole(namespace, resource, role, verbs); err != nil {
				return err
			}

			bindingName := bindingMeta.GetName()
			if b, _ := m.rbLister.Get(namespace, bindingName); b != nil {
				continue
			}

			_, err := bindingCli.Create(&v1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: bindingName,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: bindingTypeMeta.GetAPIVersion(),
							Kind:       bindingTypeMeta.GetKind(),
							Name:       bindingMeta.GetName(),
							UID:        bindingMeta.GetUID(),
						},
					},
				},
				Subjects: []v1.Subject{subject},
				RoleRef: v1.RoleRef{
					Kind: "Role",
					Name: role.Name,
				},
			})

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// If the roleTemplate has rules granting access to a managment plane resource, return the verbs for those rules
func (m *manager) checkForManagementPlaneRules(role *v3.RoleTemplate, managmentPlaneResource string) map[string]bool {
	var rules []v1.PolicyRule
	if !role.External {
		rules = role.Rules
	}

	verbs := map[string]bool{}
	for _, rule := range rules {
		if (slice.ContainsString(rule.Resources, managmentPlaneResource) || slice.ContainsString(rule.Resources, "*")) && len(rule.ResourceNames) == 0 {
			for _, v := range rule.Verbs {
				verbs[v] = true
			}
		}
	}

	return verbs
}

func (m *manager) reconcileManagementPlaneRole(namespace, resource string, rt *v3.RoleTemplate, newVerbs map[string]bool) error {
	roleCli := m.mgmt.RBAC.Roles(namespace)
	if role, err := m.rLister.Get(namespace, rt.Name); err == nil && role != nil {
		currentVerbs := map[string]bool{}
		for _, rule := range role.Rules {
			if slice.ContainsString(rule.Resources, resource) {
				for _, v := range rule.Verbs {
					currentVerbs[v] = true
				}
			}
		}

		if !reflect.DeepEqual(currentVerbs, newVerbs) {
			role = role.DeepCopy()
			added := false
			for i, rule := range role.Rules {
				if slice.ContainsString(rule.Resources, resource) {
					role.Rules[i] = buildRule(resource, newVerbs)
					added = true
				}
			}
			if !added {
				role.Rules = append(role.Rules, buildRule(resource, newVerbs))
			}
			_, err := roleCli.Update(role)
			return err
		}
		return nil
	}

	rules := []v1.PolicyRule{buildRule(resource, newVerbs)}
	_, err := roleCli.Create(&v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: rt.Name,
		},
		Rules: rules,
	})
	if err != nil {
		return errors.Wrapf(err, "couldn't create role %v", rt.Name)
	}

	return nil
}

func (m *manager) gatherRoleTemplates(rt *v3.RoleTemplate, roleTemplates map[string]*v3.RoleTemplate) error {
	roleTemplates[rt.Name] = rt

	for _, rtName := range rt.RoleTemplateNames {
		subRT, err := m.rtLister.Get("", rtName)
		if err != nil {
			return errors.Wrapf(err, "couldn't get RoleTemplate %s", rtName)
		}
		if err := m.gatherRoleTemplates(subRT, roleTemplates); err != nil {
			return errors.Wrapf(err, "couldn't gather RoleTemplate %s", rtName)
		}
	}

	return nil
}

func buildRule(resource string, verbs map[string]bool) v1.PolicyRule {
	var vs []string
	for v := range verbs {
		vs = append(vs, v)
	}
	return v1.PolicyRule{
		Resources: []string{resource},
		Verbs:     vs,
		APIGroups: []string{"*"},
	}
}
