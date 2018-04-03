package auth

import (
	"reflect"

	"github.com/pkg/errors"
	"github.com/rancher/norman/clientbase"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types/slice"
	v13 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	"github.com/rancher/types/user"
	"k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

func newRTBLifecycles(management *config.ManagementContext) (*prtbLifecycle, *crtbLifecycle) {
	crbInformer := management.RBAC.ClusterRoleBindings("").Controller().Informer()
	crbIndexers := map[string]cache.IndexFunc{
		rbByRoleAndSubjectIndex: rbByRoleAndSubject,
	}
	crbInformer.AddIndexers(crbIndexers)

	rbInformer := management.RBAC.RoleBindings("").Controller().Informer()
	rbIndexers := map[string]cache.IndexFunc{
		rbByOwnerIndex:          rbByOwner,
		rbByRoleAndSubjectIndex: rbByRoleAndSubject,
	}
	rbInformer.AddIndexers(rbIndexers)

	mgr := &manager{
		mgmt:          management,
		projectLister: management.Management.Projects("").Controller().Lister(),
		crbLister:     management.RBAC.ClusterRoleBindings("").Controller().Lister(),
		crLister:      management.RBAC.ClusterRoles("").Controller().Lister(),
		rLister:       management.RBAC.Roles("").Controller().Lister(),
		rbLister:      management.RBAC.RoleBindings("").Controller().Lister(),
		rtLister:      management.Management.RoleTemplates("").Controller().Lister(),
		nsLister:      management.Core.Namespaces("").Controller().Lister(),
		rbIndexer:     rbInformer.GetIndexer(),
		crbIndexer:    crbInformer.GetIndexer(),
		userMGR:       management.UserManager,
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

type manager struct {
	projectLister v3.ProjectLister
	crLister      typesrbacv1.ClusterRoleLister
	rLister       typesrbacv1.RoleLister
	rbLister      typesrbacv1.RoleBindingLister
	crbLister     typesrbacv1.ClusterRoleBindingLister
	rtLister      v3.RoleTemplateLister
	nsLister      v13.NamespaceLister
	rbIndexer     cache.Indexer
	crbIndexer    cache.Indexer
	mgmt          *config.ManagementContext
	userMGR       user.Manager
}

// When a CRTB is created that gives a subject some permissions in a project or cluster, we need to create a "membership" binding
// that gives the subject access to the the cluster custom resource itself
// This is painfully similar to ensureProjectMemberBinding, but making one function that handles both is overly complex
func (m *manager) ensureClusterMembershipBinding(roleName, rtbUID string, cluster *v3.Cluster, makeOwner bool, subject v1.Subject) error {
	if err := m.createClusterMembershipRole(roleName, cluster, makeOwner); err != nil {
		return err
	}

	key := rbRoleSubjectKey(roleName, subject)
	set := labels.Set(map[string]string{rtbUID: membershipBindingOwner})
	crbs, err := m.crbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}
	var crb *v1.ClusterRoleBinding
	for _, iCRB := range crbs {
		if len(iCRB.Subjects) != 1 {
			iKey := rbRoleSubjectKey(iCRB.RoleRef.Name, iCRB.Subjects[0])
			if iKey == key {
				crb = iCRB
				continue
			}
		}
		if err := m.reconcileClusterMembershipBindingForDelete(roleName, rtbUID); err != nil {
			return err
		}
	}

	if crb != nil {
		return nil
	}

	objs, err := m.crbIndexer.ByIndex(rbByRoleAndSubjectIndex, key)
	if err != nil {
		return err
	}

	if len(objs) == 0 {
		_, err := m.mgmt.RBAC.ClusterRoleBindings("").Create(&v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "clusterrolebinding-",
				Labels: map[string]string{
					rtbUID: membershipBindingOwner,
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

	crb, _ = objs[0].(*v1.ClusterRoleBinding)
	for owner := range crb.Labels {
		if rtbUID == owner {
			return nil
		}
	}

	crb = crb.DeepCopy()
	if crb.Labels == nil {
		crb.Labels = map[string]string{}
	}
	crb.Labels[rtbUID] = membershipBindingOwner
	_, err = m.mgmt.RBAC.ClusterRoleBindings("").Update(crb)
	return err
}

// When a PRTB is created that gives a subject some permissions in a project or cluster, we need to create a "membership" binding
// that gives the subject access to the the project/cluster custom resource itself
func (m *manager) ensureProjectMembershipBinding(roleName, rtbUID, namespace string, project *v3.Project, makeOwner bool, subject v1.Subject) error {
	if err := m.createProjectMembershipRole(roleName, namespace, project, makeOwner); err != nil {
		return err
	}

	key := rbRoleSubjectKey(roleName, subject)
	set := labels.Set(map[string]string{rtbUID: membershipBindingOwner})
	rbs, err := m.rbLister.List("", set.AsSelector())
	if err != nil {
		return err
	}
	var rb *v1.RoleBinding
	for _, iRB := range rbs {
		if len(iRB.Subjects) != 1 {
			iKey := rbRoleSubjectKey(iRB.RoleRef.Name, iRB.Subjects[0])
			if iKey == key {
				rb = iRB
				continue
			}
		}
		if err := m.reconcileProjectMembershipBindingForDelete(namespace, roleName, rtbUID); err != nil {
			return err
		}
	}

	if rb != nil {
		return nil
	}

	objs, err := m.crbIndexer.ByIndex(rbByRoleAndSubjectIndex, key)
	if err != nil {
		return err
	}

	if len(objs) == 0 {
		_, err := m.mgmt.RBAC.RoleBindings(namespace).Create(&v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "rolebinding-",
				Labels: map[string]string{
					rtbUID: membershipBindingOwner,
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

	rb, _ = objs[0].(*v1.RoleBinding)
	for owner := range rb.Labels {
		if rtbUID == owner {
			return nil
		}
	}

	rb = rb.DeepCopy()
	if rb.Labels == nil {
		rb.Labels = map[string]string{}
	}
	rb.Labels[rtbUID] = membershipBindingOwner
	_, err = m.mgmt.RBAC.RoleBindings(namespace).Update(rb)
	return err
}

// Creates a role that lets the bound subject see (if they are an ordinary member) the project or cluster in the mgmt api
// (or CRUD the project/cluster if they are an owner)
func (m *manager) createClusterMembershipRole(roleName string, cluster *v3.Cluster, makeOwner bool) error {
	if cr, _ := m.crLister.Get("", roleName); cr == nil {
		return m.createMembershipRole(clusterResource, roleName, makeOwner, cluster, m.mgmt.RBAC.ClusterRoles("").ObjectClient())
	}
	return nil
}

// Creates a role that lets the bound subject see (if they are an ordinary member) the project in the mgmt api
// (or CRUD the project if they are an owner)
func (m *manager) createProjectMembershipRole(roleName, namespace string, project *v3.Project, makeOwner bool) error {
	if cr, _ := m.rLister.Get(namespace, roleName); cr == nil {
		return m.createMembershipRole(projectResource, roleName, makeOwner, project, m.mgmt.RBAC.Roles(namespace).ObjectClient())
	}
	return nil
}

func (m *manager) createMembershipRole(resourceType, roleName string, makeOwner bool, ownerObject interface{}, client *objectclient.ObjectClient) error {
	metaObj, err := meta.Accessor(ownerObject)
	if err != nil {
		return err
	}
	typeMeta, err := meta.TypeAccessor(ownerObject)
	if err != nil {
		return err
	}
	rules := []v1.PolicyRule{
		{
			APIGroups:     []string{"management.cattle.io"},
			Resources:     []string{resourceType},
			ResourceNames: []string{metaObj.GetName()},
			Verbs:         []string{"get"},
		},
	}

	if makeOwner {
		rules[0].Verbs = []string{"*"}
	} else {
		rules[0].Verbs = []string{"get"}
	}
	_, err = client.Create(&v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: typeMeta.GetAPIVersion(),
					Kind:       typeMeta.GetKind(),
					Name:       metaObj.GetName(),
					UID:        metaObj.GetUID(),
				},
			},
		},
		Rules: rules,
	})
	return err
}

// The CRTB has been deleted or modified, either delete or update the membership binding so that the subject
// is removed from the cluster if they should be
func (m *manager) reconcileClusterMembershipBindingForDelete(roleToKeep, rtbUID string) error {
	list := func(ns string, selector labels.Selector) ([]runtime.Object, error) {
		rbs, err := m.crbLister.List(ns, selector)
		if err != nil {
			return nil, err
		}

		var items []runtime.Object
		for _, rb := range rbs {
			items = append(items, rb.DeepCopy())
		}
		return items, nil
	}

	convert := func(i interface{}) string {
		rb, _ := i.(*v1.ClusterRoleBinding)
		return rb.RoleRef.Name
	}

	return m.reconcileMembershipBindingForDelete("", roleToKeep, rtbUID, list, convert, m.mgmt.RBAC.ClusterRoleBindings("").ObjectClient())
}

// The PRTB has been deleted, either delete or update the project membership binding so that the subject
// is removed from the project if they should be
func (m *manager) reconcileProjectMembershipBindingForDelete(namespace, roleToKeep, rtbUID string) error {
	list := func(ns string, selector labels.Selector) ([]runtime.Object, error) {
		rbs, err := m.rbLister.List(ns, selector)
		if err != nil {
			return nil, err
		}

		var items []runtime.Object
		for _, rb := range rbs {
			items = append(items, rb.DeepCopy())
		}
		return items, nil
	}

	convert := func(i interface{}) string {
		rb, _ := i.(*v1.RoleBinding)
		return rb.RoleRef.Name
	}

	return m.reconcileMembershipBindingForDelete(namespace, roleToKeep, rtbUID, list, convert, m.mgmt.RBAC.RoleBindings(namespace).ObjectClient())
}

type listFn func(ns string, selector labels.Selector) ([]runtime.Object, error)
type convertFn func(i interface{}) string

func (m *manager) reconcileMembershipBindingForDelete(namespace, roleToKeep, rtbUID string, list listFn, convert convertFn, client *objectclient.ObjectClient) error {
	set := labels.Set(map[string]string{rtbUID: membershipBindingOwner})
	roleBindings, err := list(namespace, set.AsSelector())
	if err != nil {
		return err
	}

	for _, rb := range roleBindings {
		objMeta, err := meta.Accessor(rb)
		if err != nil {
			return err
		}

		roleName := convert(rb)
		if roleName == roleToKeep {
			continue
		}

		for k, v := range objMeta.GetLabels() {
			if k == rtbUID && v == membershipBindingOwner {
				delete(objMeta.GetLabels(), k)
			}
		}

		if len(objMeta.GetLabels()) == 0 {
			if err := client.Delete(objMeta.GetName(), &metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
		} else {
			if _, err := client.Update(objMeta.GetName(), rb); err != nil {
				return err
			}
		}
	}

	return nil
}

// Certain resources (projects, machines, prtbs, crtbs, clusterevents, etc) exist in the mangement plane but are scoped to clusters or
// projects. They need special RBAC handling because the need to be authorized just inside of the namespace that backs the project
// or cluster they belong to.
func (m *manager) grantManagementPlanePrivileges(roleTemplateName string, resources []string, subject v1.Subject, binding interface{}) error {
	bindingMeta, err := meta.Accessor(binding)
	if err != nil {
		return err
	}
	bindingTypeMeta, err := meta.TypeAccessor(binding)
	if err != nil {
		return err
	}
	namespace := bindingMeta.GetNamespace()

	roles, err := m.gatherAndDedupeRoles(roleTemplateName)
	if err != nil {
		return err
	}

	desiredRBs := map[string]*v1.RoleBinding{}
	roleBindings := m.mgmt.RBAC.RoleBindings(namespace)
	for _, role := range roles {
		for _, resource := range resources {
			verbs, err := m.checkForManagementPlaneRules(role, resource)
			if err != nil {
				return err
			}
			if len(verbs) > 0 {
				if err := m.reconcileManagementPlaneRole(namespace, resource, role, verbs); err != nil {
					return err
				}

				bindingName := bindingMeta.GetName() + "-" + role.Name
				if _, ok := desiredRBs[bindingName]; !ok {
					desiredRBs[bindingName] = &v1.RoleBinding{
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
					}
				}
			}
		}
	}

	currentRBs := map[string]*v1.RoleBinding{}
	current, err := m.rbIndexer.ByIndex(rbByOwnerIndex, string(bindingMeta.GetUID()))
	if err != nil {
		return err
	}
	for _, c := range current {
		rb := c.(*v1.RoleBinding)
		currentRBs[rb.Name] = rb
	}

	return m.reconcileDesiredMGMTPlaneRoleBindings(currentRBs, desiredRBs, roleBindings)
}

// grantManagementClusterScopedPrivilegesInProjectNamespace ensures that rolebindings for roles like cluster-owner (that should be able to fully
// manage all projects in a cluster) grant proper permissions to project-scoped resources. Specifically, this satisfies the use case that
// a cluster owner should be able to manage the members of all projects in their cluster
func (m *manager) grantManagementClusterScopedPrivilegesInProjectNamespace(roleTemplateName, projectNamespace string, resources []string,
	subject v1.Subject, binding *v3.ClusterRoleTemplateBinding) error {
	roles, err := m.gatherAndDedupeRoles(roleTemplateName)
	if err != nil {
		return err
	}

	desiredRBs := map[string]*v1.RoleBinding{}
	roleBindings := m.mgmt.RBAC.RoleBindings(projectNamespace)
	for _, role := range roles {
		for _, resource := range resources {
			verbs, err := m.checkForManagementPlaneRules(role, resource)
			if err != nil {
				return err
			}
			if len(verbs) > 0 {
				if err := m.reconcileManagementPlaneRole(projectNamespace, resource, role, verbs); err != nil {
					return err
				}

				bindingName := binding.Name + "-" + role.Name
				if _, ok := desiredRBs[bindingName]; !ok {
					desiredRBs[bindingName] = &v1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name: bindingName,
							Labels: map[string]string{
								string(binding.UID): crtbInProjectBindingOwner,
							},
						},
						Subjects: []v1.Subject{subject},
						RoleRef: v1.RoleRef{
							Kind: "Role",
							Name: role.Name,
						},
					}
				}
			}
		}
	}

	currentRBs := map[string]*v1.RoleBinding{}
	set := labels.Set(map[string]string{string(binding.UID): crtbInProjectBindingOwner})
	current, err := m.rbLister.List(projectNamespace, set.AsSelector())
	if err != nil {
		return err
	}
	for _, rb := range current {
		currentRBs[rb.Name] = rb
	}

	return m.reconcileDesiredMGMTPlaneRoleBindings(currentRBs, desiredRBs, roleBindings)
}

func (m *manager) gatherAndDedupeRoles(roleTemplateName string) (map[string]*v3.RoleTemplate, error) {
	rt, err := m.rtLister.Get("", roleTemplateName)
	if err != nil {
		return nil, err
	}
	allRoles := map[string]*v3.RoleTemplate{}
	if err := m.gatherRoleTemplates(rt, allRoles); err != nil {
		return nil, err
	}

	//de-dupe
	roles := map[string]*v3.RoleTemplate{}
	for _, role := range allRoles {
		roles[role.Name] = role
	}
	return roles, nil
}

func (m *manager) reconcileDesiredMGMTPlaneRoleBindings(currentRBs, desiredRBs map[string]*v1.RoleBinding, roleBindings typesrbacv1.RoleBindingInterface) error {
	rbsToDelete := map[string]bool{}
	processed := map[string]bool{}
	for _, rb := range currentRBs {
		// protect against an rb being in the list more than once (shouldn't happen, but just to be safe)
		if ok := processed[rb.Name]; ok {
			continue
		}
		processed[rb.Name] = true

		if _, ok := desiredRBs[rb.Name]; ok {
			delete(desiredRBs, rb.Name)
		} else {
			rbsToDelete[rb.Name] = true
		}
	}

	for _, rb := range desiredRBs {
		_, err := roleBindings.Create(rb)
		if err != nil {
			return err
		}
	}

	for name := range rbsToDelete {
		if err := roleBindings.Delete(name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// If the roleTemplate has rules granting access to a managment plane resource, return the verbs for those rules
func (m *manager) checkForManagementPlaneRules(role *v3.RoleTemplate, managmentPlaneResource string) (map[string]bool, error) {
	var rules []v1.PolicyRule
	if role.External {
		externalRole, err := m.crLister.Get("", role.Name)
		if err != nil && !clientbase.IsNotFound(err) {
			// dont error if it doesnt exist
			return nil, err
		}
		if externalRole != nil {
			rules = externalRole.Rules
		}
	} else {
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

	return verbs, nil
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

func buildSubjectFromRTB(binding interface{}) (v1.Subject, error) {
	var userName, groupPrincipalName, groupName, name, kind string
	if rtb, ok := binding.(*v3.ProjectRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else if rtb, ok := binding.(*v3.ClusterRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else {
		return v1.Subject{}, errors.Errorf("unrecognized roleTemplateBinding type: %v", binding)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return v1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", binding)
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return v1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", binding)
		}
		name = groupName
		kind = "Group"
	}

	if name == "" {
		return v1.Subject{}, errors.Errorf("roletemplatebinding doesn't have any subject fields set: %v", binding)
	}

	return v1.Subject{
		Kind: kind,
		Name: name,
	}, nil
}
