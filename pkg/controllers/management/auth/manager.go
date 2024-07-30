package auth

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rancher/norman/objectclient"
	"github.com/rancher/norman/types/slice"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/controllers/managementuser/rbac"
	"github.com/rancher/rancher/pkg/features"
	v13 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	typesrbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	pkgrbac "github.com/rancher/rancher/pkg/rbac"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/rancher/pkg/user"
	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"
)

const (
	clusterContext = "cluster"
	projectContext = "project"

	rolesCircularSoftLimit = 100
	rolesCircularHardLimit = 500
	clusterNameLabel       = "cluster.cattle.io/name"
)

var commonClusterAndProjectMgmtPlaneResources = map[string]bool{
	"catalogtemplates":        true,
	"catalogtemplateversions": true,
}

func newRTBLifecycles(management *config.ManagementContext) (*prtbLifecycle, *crtbLifecycle) {
	crbInformer := management.RBAC.ClusterRoleBindings("").Controller().Informer()
	rbInformer := management.RBAC.RoleBindings("").Controller().Informer()

	prtb := &prtbLifecycle{
		mgr: &manager{
			mgmt:          management,
			projectLister: management.Management.Projects("").Controller().Lister(),
			crbLister:     management.RBAC.ClusterRoleBindings("").Controller().Lister(),
			crbClient:     management.RBAC.ClusterRoleBindings(""),
			crLister:      management.RBAC.ClusterRoles("").Controller().Lister(),
			rLister:       management.RBAC.Roles("").Controller().Lister(),
			rbLister:      management.RBAC.RoleBindings("").Controller().Lister(),
			rbClient:      management.RBAC.RoleBindings(""),
			rtLister:      management.Management.RoleTemplates("").Controller().Lister(),
			userLister:    management.Management.Users("").Controller().Lister(),
			rbIndexer:     rbInformer.GetIndexer(),
			crbIndexer:    crbInformer.GetIndexer(),
			userMGR:       management.UserManager,
			controller:    ptrbMGMTController,
			prtbs:         management.Management.ProjectRoleTemplateBindings(""),
		},
		projectLister: management.Management.Projects("").Controller().Lister(),
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	crtb := &crtbLifecycle{
		mgr: &manager{
			mgmt:          management,
			projectLister: management.Management.Projects("").Controller().Lister(),
			crbLister:     management.RBAC.ClusterRoleBindings("").Controller().Lister(),
			crbClient:     management.RBAC.ClusterRoleBindings(""),
			crLister:      management.RBAC.ClusterRoles("").Controller().Lister(),
			rLister:       management.RBAC.Roles("").Controller().Lister(),
			rbLister:      management.RBAC.RoleBindings("").Controller().Lister(),
			rbClient:      management.RBAC.RoleBindings(""),
			rtLister:      management.Management.RoleTemplates("").Controller().Lister(),
			userLister:    management.Management.Users("").Controller().Lister(),
			rbIndexer:     rbInformer.GetIndexer(),
			crbIndexer:    crbInformer.GetIndexer(),
			userMGR:       management.UserManager,
			controller:    ctrbMGMTController,
			crtbs:         management.Management.ClusterRoleTemplateBindings(""),
		},
		clusterLister: management.Management.Clusters("").Controller().Lister(),
	}
	return prtb, crtb
}

type manager struct {
	projectLister  v3.ProjectLister
	crLister       typesrbacv1.ClusterRoleLister
	rLister        typesrbacv1.RoleLister
	rbLister       typesrbacv1.RoleBindingLister
	rbClient       typesrbacv1.RoleBindingInterface
	crbLister      typesrbacv1.ClusterRoleBindingLister
	crbClient      typesrbacv1.ClusterRoleBindingInterface
	rtLister       v3.RoleTemplateLister
	nsLister       v13.NamespaceLister
	userLister     v3.UserLister
	rbIndexer      cache.Indexer
	crbIndexer     cache.Indexer
	mgmt           *config.ManagementContext
	userMGR        user.Manager
	controller     string
	clusterManager *clustermanager.Manager
	crtbs          v3.ClusterRoleTemplateBindingInterface
	prtbs          v3.ProjectRoleTemplateBindingInterface
}

// When a CRTB is created that gives a subject some permissions in a project or cluster, we need to create a "membership" binding
// that gives the subject access to the the cluster custom resource itself
// This is painfully similar to ensureProjectMemberBinding, but making one function that handles both is overly complex
func (m *manager) ensureClusterMembershipBinding(roleName, rtbNsAndName string, cluster *v3.Cluster, makeOwner bool, subject v1.Subject) error {
	if err := m.createClusterMembershipRole(roleName, cluster, makeOwner); err != nil {
		return err
	}

	key := rbRoleSubjectKey(roleName, subject)
	crbs, err := m.crbIndexer.ByIndex(membershipBindingOwnerIndex, "/"+rtbNsAndName)
	if err != nil {
		return err
	}
	var crb *v1.ClusterRoleBinding
	for _, iCRB := range crbs {
		if iCRB, ok := iCRB.(*v1.ClusterRoleBinding); ok {
			if len(iCRB.Subjects) != 1 {
				iKey := rbRoleSubjectKey(iCRB.RoleRef.Name, iCRB.Subjects[0])
				if iKey == key {
					crb = iCRB
					continue
				}
			}
		}
	}

	if err := m.reconcileClusterMembershipBindingForDelete(roleName, rtbNsAndName); err != nil {
		return err
	}

	if crb != nil {
		return nil
	}

	objs, err := m.crbIndexer.ByIndex(rbByRoleAndSubjectIndex, key)
	if err != nil {
		return err
	}

	if len(objs) == 0 {
		logrus.Infof("[%v] Creating clusterRoleBinding for membership in cluster %v for subject %v", m.controller, cluster.Name, subject.Name)
		roleRef := v1.RoleRef{
			Kind: "ClusterRole",
			Name: roleName,
		}
		crbName := pkgrbac.NameForClusterRoleBinding(roleRef, subject) // use deterministic name for crb
		_, err = m.mgmt.RBAC.ClusterRoleBindings("").Create(&v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:        crbName,
				Annotations: map[string]string{clusterNameLabel: cluster.Name},
				Labels: map[string]string{
					rtbNsAndName: MembershipBindingOwner,
				},
			},
			Subjects: []v1.Subject{subject},
			RoleRef:  roleRef,
		})
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		// if the binding exists but was not found in the index, manually retrieve it so that we can add appropriate labels
		crb, err := m.mgmt.RBAC.ClusterRoleBindings("").Get(crbName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		objs = append(objs, crb)
	}

	crb, _ = objs[0].(*v1.ClusterRoleBinding)
	for owner := range crb.Labels {
		if rtbNsAndName == owner {
			return nil
		}
	}

	crb = crb.DeepCopy()
	if crb.Labels == nil {
		crb.Labels = map[string]string{}
	}
	crb.Labels[rtbNsAndName] = MembershipBindingOwner
	logrus.Infof("[%v] Updating clusterRoleBinding %v for cluster membership in cluster %v for subject %v", m.controller, crb.Name, cluster.Name, subject.Name)
	_, err = m.mgmt.RBAC.ClusterRoleBindings("").Update(crb)
	return err
}

// When a PRTB is created that gives a subject some permissions in a project or cluster, we need to create a "membership" binding
// that gives the subject access to the the project/cluster custom resource itself
func (m *manager) ensureProjectMembershipBinding(roleName, rtbNsAndName, namespace string, project *v3.Project, makeOwner bool, subject v1.Subject) error {
	if err := m.createProjectMembershipRole(roleName, namespace, project, makeOwner); err != nil {
		return err
	}

	key := rbRoleSubjectKey(roleName, subject)
	rbs, err := m.rbIndexer.ByIndex(membershipBindingOwnerIndex, namespace+"/"+rtbNsAndName)
	if err != nil {
		return err
	}
	var rb *v1.RoleBinding
	for _, iRB := range rbs {
		if iRB, ok := iRB.(*v1.RoleBinding); ok {
			if len(iRB.Subjects) != 1 {
				iKey := rbRoleSubjectKey(iRB.RoleRef.Name, iRB.Subjects[0])
				if iKey == key {
					rb = iRB
					continue
				}
			}
		}
	}

	if err := m.reconcileProjectMembershipBindingForDelete(namespace, roleName, rtbNsAndName); err != nil {
		return err
	}

	if rb != nil {
		return nil
	}

	objs, err := m.rbIndexer.ByIndex(rbByRoleAndSubjectIndex, key)
	if err != nil {
		return err
	}

	if len(objs) == 0 {
		logrus.Infof("[%v] Creating roleBinding for membership in project %v for subject %v", m.controller, project.Name, subject.Name)
		roleRef := v1.RoleRef{
			Kind: "Role",
			Name: roleName,
		}
		// use deterministic name for rb
		rbName := pkgrbac.NameForRoleBinding(namespace, roleRef, subject)
		_, err = m.mgmt.RBAC.RoleBindings(namespace).Create(&v1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: rbName,
				Labels: map[string]string{
					rtbNsAndName: MembershipBindingOwner,
				},
			},
			Subjects: []v1.Subject{subject},
			RoleRef:  roleRef,
		})
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		// if the binding already exists but was not found in the index, manually retrieve it so that we can add appropriate labels
		rb, err := m.mgmt.RBAC.RoleBindings(namespace).Get(rbName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		objs = append(objs, rb)
	}

	rb, _ = objs[0].(*v1.RoleBinding)
	for owner := range rb.Labels {
		if rtbNsAndName == owner {
			return nil
		}
	}

	rb = rb.DeepCopy()
	if rb.Labels == nil {
		rb.Labels = map[string]string{}
	}
	rb.Labels[rtbNsAndName] = MembershipBindingOwner
	logrus.Infof("[%v] Updating roleBinding %v for project membership in project %v for subject %v", m.controller, rb.Name, project.Name, subject.Name)
	_, err = m.mgmt.RBAC.RoleBindings(namespace).Update(rb)
	return err
}

// Creates a role that lets the bound subject see (if they are an ordinary member) the project or cluster in the mgmt api
// (or CRUD the project/cluster if they are an owner)
func (m *manager) createClusterMembershipRole(roleName string, cluster *v3.Cluster, makeOwner bool) error {
	if cr, _ := m.crLister.Get("", roleName); cr == nil {
		return m.createMembershipRole(clusterResource, roleName, makeOwner, cluster, m.mgmt.RBAC.ClusterRoles("").ObjectClient(), true)
	}
	return nil
}

// Creates a role that lets the bound subject see (if they are an ordinary member) the project in the mgmt api
// (or CRUD the project if they are an owner)
func (m *manager) createProjectMembershipRole(roleName, namespace string, project *v3.Project, makeOwner bool) error {
	if cr, _ := m.rLister.Get(namespace, roleName); cr == nil {
		return m.createMembershipRole(projectResource, roleName, makeOwner, project, m.mgmt.RBAC.Roles(namespace).ObjectClient(), false)
	}
	return nil
}

func (m *manager) createMembershipRole(resourceType, roleName string, makeOwner bool, ownerObject interface{}, client *objectclient.ObjectClient, clusterRole bool) error {
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
	logrus.Infof("[%v] Creating role/clusterRole %v", m.controller, roleName)
	var toCreate runtime.Object
	objectMeta := metav1.ObjectMeta{
		Name: roleName,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: typeMeta.GetAPIVersion(),
				Kind:       typeMeta.GetKind(),
				Name:       metaObj.GetName(),
				UID:        metaObj.GetUID(),
			},
		},
	}
	if clusterRole {
		objectMeta.Annotations = map[string]string{clusterNameLabel: metaObj.GetName()}
		toCreate = &v1.ClusterRole{
			ObjectMeta: objectMeta,
			Rules:      rules,
		}
	} else {
		toCreate = &v1.Role{
			ObjectMeta: objectMeta,
			Rules:      rules,
		}
	}
	_, err = client.Create(toCreate)
	return err
}

// The CRTB has been deleted or modified, either delete or update the membership binding so that the subject
// is removed from the cluster if they should be
func (m *manager) reconcileClusterMembershipBindingForDelete(roleToKeep, rtbNsAndName string) error {
	convert := func(i interface{}) string {
		rb, _ := i.(*v1.ClusterRoleBinding)
		return rb.RoleRef.Name
	}

	return m.reconcileMembershipBindingForDelete("", roleToKeep, rtbNsAndName, m.crbIndexer, convert, m.mgmt.RBAC.ClusterRoleBindings("").ObjectClient())
}

// The PRTB has been deleted, either delete or update the project membership binding so that the subject
// is removed from the project if they should be
func (m *manager) reconcileProjectMembershipBindingForDelete(namespace, roleToKeep, rtbNsAndName string) error {
	convert := func(i interface{}) string {
		rb, _ := i.(*v1.RoleBinding)
		return rb.RoleRef.Name
	}

	return m.reconcileMembershipBindingForDelete(namespace, roleToKeep, rtbNsAndName, m.rbIndexer, convert, m.mgmt.RBAC.RoleBindings(namespace).ObjectClient())
}

type convertFn func(i interface{}) string

func (m *manager) reconcileMembershipBindingForDelete(namespace, roleToKeep, rtbNsAndName string, index cache.Indexer, convert convertFn, client *objectclient.ObjectClient) error {
	roleBindings, err := index.ByIndex(membershipBindingOwnerIndex, namespace+"/"+rtbNsAndName)
	if err != nil {
		return err
	}

	for _, elt := range roleBindings {
		obj, ok := elt.(runtime.Object)
		if !ok {
			continue
		}

		// copy so we don't modify object in cache
		objCopy := obj.DeepCopyObject()

		objMeta, err := meta.Accessor(objCopy)
		if err != nil {
			return err
		}

		roleName := convert(objCopy)
		if roleName == roleToKeep {
			continue
		}
		var otherOwners bool
		for k, v := range objMeta.GetLabels() {
			if k == rtbNsAndName && v == MembershipBindingOwner {
				delete(objMeta.GetLabels(), k)
			} else if v == MembershipBindingOwner {
				// Another rtb is also linked to this roleBinding so don't delete
				otherOwners = true
			}
		}

		if !otherOwners {
			logrus.Infof("[%v] Deleting roleBinding %v", m.controller, objMeta.GetName())
			if err := client.Delete(objMeta.GetName(), &metav1.DeleteOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return err
			}
		} else {
			logrus.Infof("[%v] Updating owner label for roleBinding %v", m.controller, objMeta.GetName())
			if _, err := client.Update(objMeta.GetName(), objCopy); err != nil {
				return err
			}
		}
	}

	return nil
}

// removeAuthV2Permissions finds any roleBindings based off the owner annotation from the incoming owner.
// This is similar to an ownerReference but this is used across namespaces which ownerReferences does not support.
func (m *manager) removeAuthV2Permissions(setID string, owner runtime.Object) error {
	// Get the selector for the dependent roleBindings
	selector, err := apply.GetSelectorFromOwner(setID, owner)
	if err != nil {
		return err
	}

	roleBindings, err := m.rbLister.List("", selector)
	if err != nil {
		return err
	}

	var returnErr error
	for _, binding := range roleBindings {
		err := m.rbClient.DeleteNamespaced(binding.Namespace, binding.Name, &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			// Combine all errors so we try our best to delete everything in the first run
			returnErr = multierror.Append(returnErr, err)
		}
	}

	return returnErr
}

// Certain resources (projects, machines, prtbs, crtbs, clusterevents, etc) exist in the mangement plane but are scoped to clusters or
// projects. They need special RBAC handling because the need to be authorized just inside of the namespace that backs the project
// or cluster they belong to.
func (m *manager) grantManagementPlanePrivileges(roleTemplateName string, resources map[string]string, subject v1.Subject, binding interface{}) error {
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
		resourceToVerbs := map[string]map[string]string{}
		for resource, apiGroup := range resources {
			verbs, err := m.checkForManagementPlaneRules(role, resource, apiGroup)
			if err != nil {
				return err
			}
			if len(verbs) > 0 {
				resourceToVerbs[resource] = verbs
				bindingName := bindingMeta.GetName() + "-" + role.Name
				if _, ok := desiredRBs[bindingName]; !ok {
					desiredRBs[bindingName] = &v1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      bindingName,
							Namespace: namespace,
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
		if len(resourceToVerbs) > 0 {
			if err := m.reconcileManagementPlaneRole(namespace, resourceToVerbs, role); err != nil {
				return err
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
func (m *manager) grantManagementClusterScopedPrivilegesInProjectNamespace(roleTemplateName, projectNamespace string, resources map[string]string,
	subject v1.Subject, binding *v3.ClusterRoleTemplateBinding) error {
	roles, err := m.gatherAndDedupeRoles(roleTemplateName)
	if err != nil {
		return err
	}

	desiredRBs := map[string]*v1.RoleBinding{}
	roleBindings := m.mgmt.RBAC.RoleBindings(projectNamespace)
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, role := range roles {
		resourceToVerbs := map[string]map[string]string{}
		for resource, apiGroup := range resources {
			// Adding this check, because we want cluster-owners to have access to catalogtemplates/versions of all projects, but no other cluster roles
			// need to access catalogtemplates of projects they do not belong to
			if !role.Administrative && commonClusterAndProjectMgmtPlaneResources[resource] {
				continue

			}
			verbs, err := m.checkForManagementPlaneRules(role, resource, apiGroup)
			if err != nil {
				return err
			}
			if len(verbs) > 0 {
				resourceToVerbs[resource] = verbs

				bindingName := binding.Name + "-" + role.Name
				if _, ok := desiredRBs[bindingName]; !ok {
					desiredRBs[bindingName] = &v1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      bindingName,
							Namespace: projectNamespace,
							Labels: map[string]string{
								bindingKey: CrtbInProjectBindingOwner,
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
		if len(resourceToVerbs) > 0 {
			if err := m.reconcileManagementPlaneRole(projectNamespace, resourceToVerbs, role); err != nil {
				return err
			}
		}
	}

	currentRBs := map[string]*v1.RoleBinding{}
	set := labels.Set(map[string]string{bindingKey: CrtbInProjectBindingOwner})
	current, err := m.rbLister.List(projectNamespace, set.AsSelector())
	if err != nil {
		return err
	}
	for _, rb := range current {
		currentRBs[rb.Name] = rb
	}

	return m.reconcileDesiredMGMTPlaneRoleBindings(currentRBs, desiredRBs, roleBindings)
}

// grantManagementProjectScopedPrivilegesInClusterNamespace ensures that project roles grant permissions to certain cluster-scoped
// resources(notifier, clusterpipelines). These resources exists in cluster namespace but need to be shared between projects.
func (m *manager) grantManagementProjectScopedPrivilegesInClusterNamespace(roleTemplateName, clusterNamespace string, resources map[string]string,
	subject v1.Subject, binding *v3.ProjectRoleTemplateBinding) error {
	roles, err := m.gatherAndDedupeRoles(roleTemplateName)
	if err != nil {
		return err
	}

	desiredRBs := map[string]*v1.RoleBinding{}
	roleBindings := m.mgmt.RBAC.RoleBindings(clusterNamespace)
	bindingKey := pkgrbac.GetRTBLabel(binding.ObjectMeta)
	for _, role := range roles {
		resourceToVerbs := map[string]map[string]string{}
		for resource, apiGroup := range resources {
			verbs, err := m.checkForManagementPlaneRules(role, resource, apiGroup)
			if err != nil {
				return err
			}
			if len(verbs) > 0 {
				resourceToVerbs[resource] = verbs

				bindingName := fmt.Sprintf("%s-%s-%s", binding.Namespace, binding.Name, role.Name)

				if _, ok := desiredRBs[bindingName]; !ok {
					desiredRBs[bindingName] = &v1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      bindingName,
							Namespace: clusterNamespace,
							Labels: map[string]string{
								bindingKey: PrtbInClusterBindingOwner,
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
		if len(resourceToVerbs) > 0 {
			if err := m.reconcileManagementPlaneRole(clusterNamespace, resourceToVerbs, role); err != nil {
				return err
			}
		}
	}

	currentRBs := map[string]*v1.RoleBinding{}
	set := labels.Set(map[string]string{bindingKey: PrtbInClusterBindingOwner})
	current, err := m.rbLister.List(clusterNamespace, set.AsSelector())
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

	//toLower
	rbac.ToLowerRoleTemplates(roles)
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
		logrus.Infof("[%v] Creating roleBinding for subject %v with role %v in namespace %v", m.controller, rb.Subjects[0].Name, rb.RoleRef.Name, rb.Namespace)
		_, err := roleBindings.Create(rb)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	for name := range rbsToDelete {
		logrus.Infof("[%v] Deleting roleBinding %v", m.controller, name)
		if err := roleBindings.Delete(name, &metav1.DeleteOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// If the roleTemplate has rules granting access to a management plane resource, return the verbs for those rules
func (m *manager) checkForManagementPlaneRules(role *v3.RoleTemplate, managementPlaneResource string, apiGroup string) (map[string]string, error) {
	var rules []v1.PolicyRule
	if role.External {
		if features.ExternalRules.Enabled() && role.ExternalRules != nil {
			rules = append(rules, role.ExternalRules...)
		} else {
			externalRole, err := m.crLister.Get("", role.Name)
			if err != nil && !apierrors.IsNotFound(err) {
				// dont error if it doesnt exist
				return nil, err
			}
			if externalRole != nil {
				rules = externalRole.Rules
			}
		}
	} else {
		rules = role.Rules
	}

	verbs := map[string]string{}
	for _, rule := range rules {
		if (slice.ContainsString(rule.Resources, managementPlaneResource) || slice.ContainsString(rule.Resources, "*")) && len(rule.ResourceNames) == 0 {
			if checkGroup(apiGroup, rule) {
				for _, v := range rule.Verbs {
					verbs[v] = apiGroup
				}
			}
		}
	}

	return verbs, nil
}

func checkGroup(apiGroup string, rule v1.PolicyRule) bool {
	for _, rg := range rule.APIGroups {
		if rg == apiGroup || rg == "*" {
			return true
		}
	}
	return false
}

func (m *manager) reconcileManagementPlaneRole(namespace string, resourceToVerbs map[string]map[string]string, rt *v3.RoleTemplate) error {
	roleCli := m.mgmt.RBAC.Roles(namespace)
	update := false
	if role, err := m.rLister.Get(namespace, rt.Name); err == nil && role != nil {
		newRole := role.DeepCopy()
		for resource, newVerbs := range resourceToVerbs {
			currentVerbs := map[string]string{}
			for _, rule := range role.Rules {
				if slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, "*") {
					for _, v := range rule.Verbs {
						if rule.APIGroups[0] == newVerbs[v] {
							currentVerbs[v] = rule.APIGroups[0]
						} else if rule.APIGroups[0] == "*" || newVerbs[v] == "*" {
							currentVerbs[v] = newVerbs[v]
						}
					}
				}
			}

			if !reflect.DeepEqual(currentVerbs, newVerbs) {
				update = true
				role = role.DeepCopy()
				added := false
				for i, rule := range newRole.Rules {
					if slice.ContainsString(rule.Resources, resource) || slice.ContainsString(rule.Resources, "*") {
						newRole.Rules[i] = buildRule(resource, newVerbs)
						added = true
					}
				}
				if !added {
					newRole.Rules = append(newRole.Rules, buildRule(resource, newVerbs))
				}

			}
		}
		if update {
			logrus.Infof("[%v] Updating role %v in namespace %v", m.controller, newRole.Name, namespace)
			_, err := roleCli.Update(newRole)
			return err
		}
		return nil
	}

	var rules []v1.PolicyRule
	for resource, newVerbs := range resourceToVerbs {
		rules = append(rules, buildRule(resource, newVerbs))
	}
	logrus.Infof("[%v] Creating role %v in namespace %v", m.controller, rt.Name, namespace)
	_, err := roleCli.Create(&v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name: rt.Name,
		},
		Rules: rules,
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
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

func buildRule(resource string, verbs map[string]string) v1.PolicyRule {
	var vs []string
	var apiGroup string
	for v, g := range verbs {
		vs = append(vs, v)
		// This is not efficient but our list of verbs will always be > 10 and we don't know the verbs to access the apiGroup
		// Checking for empty string also won't help since core api group is empty string
		apiGroup = g
	}

	// Sort the verbs, a map does not guarantee order
	sort.Strings(vs)

	return v1.PolicyRule{
		Resources: []string{resource},
		Verbs:     vs,
		APIGroups: []string{apiGroup},
	}
}

func (m *manager) checkReferencedRoles(roleTemplateName, roleTemplateContext string, depthCounter int) (bool, error) {
	if depthCounter == rolesCircularSoftLimit {
		logrus.Warnf("roletemplate has caused %v recursive function calls", rolesCircularSoftLimit)
	}
	if depthCounter >= rolesCircularHardLimit {
		return false, fmt.Errorf("roletemplate '%s' has caused %d recursive function calls, possible circular dependency", roleTemplateName, rolesCircularHardLimit)
	}
	roleTemplate, err := m.rtLister.Get("", roleTemplateName)
	if err != nil {
		return false, err
	}

	// Only check if we are in the same context, if the roleTemplate is from a different context then
	// it can't possibly be a owner in the callers context.
	if roleTemplate.Context != roleTemplateContext {
		return false, nil
	}

	// upon upgrades, crtb/prtbs are reconciled before roletemplates.
	// So these roles won't have the "own" verb at the time of this check added 2.4.6 onwards
	if roleTemplate.Builtin && roleTemplate.Context == projectContext && roleTemplateName == "project-owner" {
		return true, nil
	}
	if roleTemplate.Builtin && roleTemplate.Context == clusterContext && roleTemplateName == "cluster-owner" {
		return true, nil
	}

	for _, rule := range roleTemplate.Rules {
		if slice.ContainsString(rule.Resources, projectResource) || slice.ContainsString(rule.Resources, clusterResource) {
			if slice.ContainsString(rule.Verbs, "own") {
				return true, nil
			}
		}
	}
	isOwnerRole := false
	if len(roleTemplate.RoleTemplateNames) > 0 {
		depthCounter++
		// get referenced roletemplate
		for _, rtName := range roleTemplate.RoleTemplateNames {
			isOwnerRole, err = m.checkReferencedRoles(rtName, roleTemplateContext, depthCounter)
			if err != nil {
				return false, err
			}
			if isOwnerRole {
				return true, nil
			}
		}
	}
	return isOwnerRole, nil
}

func getLabelRequirements(objMeta metav1.ObjectMeta) ([]labels.Requirement, error) {
	reqUpdatedLabel, err := labels.NewRequirement(rtbLabelUpdated, selection.DoesNotExist, []string{})
	if err != nil {
		return []labels.Requirement{}, err
	}
	reqNsAndNameLabel, err := labels.NewRequirement(pkgrbac.GetRTBLabel(objMeta), selection.DoesNotExist, []string{})
	if err != nil {
		return []labels.Requirement{}, err
	}
	return []labels.Requirement{*reqUpdatedLabel, *reqNsAndNameLabel}, nil
}
