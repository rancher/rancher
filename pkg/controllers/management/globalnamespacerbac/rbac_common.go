package globalnamespacerbac

import (
	"fmt"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sort"
	"strings"

	"github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"

	k8srbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	allAccess                 = "all"
	updateAccess              = "update"
	readOnlyAccess            = "readonly"
	MultiClusterAppResource   = "multiclusterapps"
	GlobalDNSResource         = "globaldnses"
	GlobalDNSProviderResource = "globaldnsproviders"
	CloudCredentialResource   = "secrets"
	CreatorIDAnn              = "field.cattle.io/creatorId"
)

func CreateRoleAndRoleBinding(resource string, name string, UID types.UID, members []v3.Member, creatorID string,
	managementContext *config.ManagementContext, apiGroup ...string) error {
	/* Create 3 Roles containing the resource (multiclusterapp or globalDNS), and the current multiclusterapp/globalDNS in resourceNames list
	1. Role with all verbs (everything access, includes creator); name multiclusterapp.Name + "-ma" / (globalDNS.Name + "-ga")
	2. Role with "get", "list" "watch" verbs (ReadOnly access); name multiclusterapp.Name + "-mr" / (globalDNS.Name + "-gr")
	3. Role with "update" verb (Upgrade access); name multiclusterapp.Name + "-mu" / (globalDNS.Name + "-gu")
	*/
	api := []string{"management.cattle.io"}
	if len(apiGroup) > 0 {
		api = apiGroup
	}
	if _, err := createRole(resource, allAccess, name, managementContext, api); err != nil {
		return err
	}

	// Create a roleBinding referring the role with everything access, and containing creator of the multiclusterapp, along with
	// any members that have everything access
	var allAccessSubjects, readOnlyAccessSubjects, updateAccessSubjects []k8srbacv1.Subject
	allAccessSubjects = append(allAccessSubjects, k8srbacv1.Subject{Kind: "User", Name: creatorID, APIGroup: rbacv1.GroupName})
	for _, m := range members {
		s, err := buildSubjectForMember(m, managementContext)
		if err != nil {
			return err
		}
		switch m.AccessType {
		case allAccess:
			allAccessSubjects = append(allAccessSubjects, s)
		case updateAccess:
			updateAccessSubjects = append(updateAccessSubjects, s)
		case readOnlyAccess:
			readOnlyAccessSubjects = append(readOnlyAccessSubjects, s)
		default:
			readOnlyAccessSubjects = append(readOnlyAccessSubjects, s)
		}
	}

	if _, err := createRoleBinding(allAccess, name, UID, allAccessSubjects, managementContext, resource, api); err != nil {
		return err
	}

	// Check if there are members with readonly or update access; if found then create rolebindings for those
	if len(readOnlyAccessSubjects) > 0 {
		if _, err := createRole(resource, readOnlyAccess, name, managementContext, api); err != nil {
			return err
		}
		if _, err := createRoleBinding(readOnlyAccess, name, UID, readOnlyAccessSubjects, managementContext, resource, api); err != nil {
			return err
		}
	}
	if len(updateAccessSubjects) > 0 {
		if _, err := createRole(resource, updateAccess, name, managementContext, api); err != nil {
			return err
		}
		if _, err := createRoleBinding(updateAccess, name, UID, updateAccessSubjects, managementContext, resource, api); err != nil {
			return err
		}
	}
	return nil
}

func createRole(resource string, roleAccess string, resourceName string,
	managementContext *config.ManagementContext, apiGroups []string) (*k8srbacv1.Role, error) {
	roleName, verbs := getRoleNameAndVerbs(roleAccess, resourceName, resource)
	newRole := &k8srbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace.GlobalNamespace,
		},
		Rules: []k8srbacv1.PolicyRule{
			{
				APIGroups:     apiGroups,
				Resources:     []string{resource},
				ResourceNames: []string{resourceName},
				Verbs:         verbs,
			},
		},
	}
	role, err := managementContext.RBAC.Roles("").GetNamespaced(namespace.GlobalNamespace, roleName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			role, err = managementContext.RBAC.Roles("").Create(newRole)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if role != nil {
		if !reflect.DeepEqual(newRole, role) {
			toUpdate := newRole.DeepCopy()
			updated, err := managementContext.RBAC.Roles("").Update(toUpdate)
			if err != nil {
				return updated, err
			}
		}
	}
	return role, nil
}

func createRoleBinding(roleAccess string, name string, UID types.UID,
	subjects []k8srbacv1.Subject, managementContext *config.ManagementContext, resource string, apiGroups []string) (*k8srbacv1.RoleBinding, error) {
	roleName, _ := getRoleNameAndVerbs(roleAccess, name, resource)
	// we can define the rolebinding first, since if it's not already present we can call create. And if it's present then we'll
	// still need to compare the current members' list
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })

	apiVersion := "management.cattle.io/v3"
	if apiGroups[0] != "management.cattle.io" {
		apiVersion = "v1"
	}
	ownerReference := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       resource,
		Name:       name,
		UID:        UID,
	}

	newRoleBinding := &k8srbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            roleName,
			Namespace:       namespace.GlobalNamespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		RoleRef: k8srbacv1.RoleRef{
			Name: roleName,
			Kind: "Role",
		},
		Subjects: subjects,
	}
	roleBinding, err := managementContext.RBAC.RoleBindings("").GetNamespaced(namespace.GlobalNamespace, roleName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = managementContext.RBAC.RoleBindings("").Create(newRoleBinding)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if roleBinding != nil {
		if !reflect.DeepEqual(roleBinding, newRoleBinding) {
			toUpdate := newRoleBinding.DeepCopy()
			updated, err := managementContext.RBAC.RoleBindings("").Update(toUpdate)
			if err != nil {
				return updated, err
			}
		}
	}
	return newRoleBinding, nil
}

func getRoleNameAndVerbs(roleAccess string, resourceName string, resource string) (string, []string) {
	var roleName string
	var verbs []string
	switch resource {
	case MultiClusterAppResource:
		resourceName += "-m"
	case GlobalDNSResource:
		resourceName += "-g"
	}
	switch roleAccess {
	case allAccess:
		roleName = resourceName + "a"
		verbs = []string{"*"}
	case updateAccess:
		roleName = resourceName + "u"
		verbs = []string{"update", "get", "list", "watch"}
	case readOnlyAccess:
		roleName = resourceName + "r"
		verbs = []string{"get", "list", "watch"}
	}
	return roleName, verbs
}

func buildSubjectForMember(member v3.Member, managementContext *config.ManagementContext) (k8srbacv1.Subject, error) {
	var name, kind string
	member, err := checkAndSetUserFields(member, managementContext)
	if err != nil {
		return k8srbacv1.Subject{}, err
	}

	if member.UserName != "" {
		name = member.UserName
		kind = "User"
	}

	if member.GroupPrincipalName != "" {
		if name != "" {
			return k8srbacv1.Subject{}, fmt.Errorf("member %v has both username and groupPrincipalId set", member)
		}
		name = member.GroupPrincipalName
		kind = "Group"
	}

	if name == "" {
		return k8srbacv1.Subject{}, fmt.Errorf("member %v doesn't have any name fields set", member)
	}

	return k8srbacv1.Subject{
		Kind:     kind,
		Name:     name,
		APIGroup: rbacv1.GroupName,
	}, nil
}

func checkAndSetUserFields(m v3.Member, managementContext *config.ManagementContext) (v3.Member, error) {
	if m.GroupPrincipalName != "" || (m.UserPrincipalName != "" && m.UserName != "") {
		return m, nil
	}
	if m.UserPrincipalName != "" && m.UserName == "" {
		displayName := m.DisplayName
		user, err := managementContext.UserManager.EnsureUser(m.UserPrincipalName, displayName)
		if err != nil {
			return m, err
		}

		m.UserName = user.Name
		return m, nil
	}
	return m, nil
}

func GetMemberGroups(members []v3.Member) map[string]string {
	groups := make(map[string]string)
	for _, m := range members {
		if m.GroupPrincipalName != "" {
			groups[m.GroupPrincipalName] = m.AccessType
		}
	}
	return groups
}

func GetUpdatedMembers(projects []string, members []v3.Member, prtbLister v3.ProjectRoleTemplateBindingLister) ([]v3.Member, error) {
	var updatedUsers, updatedGroups, updatedMembers []v3.Member
	users := make(map[string]bool)
	groups := make(map[string]bool)
	for _, p := range projects {
		split := strings.SplitN(p, ":", 2)
		if len(split) != 2 {
			return updatedMembers, fmt.Errorf("Target project name %s is invalid", p)
		}
		projectNS := split[1]
		prtbs, err := prtbLister.List(projectNS, labels.NewSelector())
		if err != nil {
			return updatedMembers, err
		}
		prtbMap := make(map[string]bool)
		// get all PRTBs for this project
		for _, prtb := range prtbs {
			if prtb.UserPrincipalName != "" && prtb.DeletionTimestamp == nil {
				prtbMap[prtb.UserPrincipalName] = true
			} else if prtb.GroupPrincipalName != "" && prtb.DeletionTimestamp == nil {
				prtbMap[prtb.GroupPrincipalName] = true
			}
		}

		// all members should have PRTBs in this project, if not, remove them
		for _, mem := range members {
			if mem.UserPrincipalName != "" && prtbMap[mem.UserPrincipalName] && !users[mem.UserPrincipalName] {
				users[mem.UserPrincipalName] = true
				updatedUsers = append(updatedUsers, mem)
			} else if mem.GroupPrincipalName != "" && prtbMap[mem.GroupPrincipalName] && !groups[mem.GroupPrincipalName] {
				groups[mem.GroupPrincipalName] = true
				updatedGroups = append(updatedGroups, mem)
			}
		}
	}
	sort.Slice(updatedUsers, func(i, j int) bool { return updatedUsers[i].UserPrincipalName < updatedUsers[j].UserPrincipalName })
	sort.Slice(updatedGroups, func(i, j int) bool { return updatedGroups[i].GroupPrincipalName < updatedGroups[j].GroupPrincipalName })
	updatedMembers = append(updatedUsers, updatedGroups...)
	return updatedMembers, nil
}

func GetCurrentMembers(members []v3.Member) []v3.Member {
	var memberUsers, memberGroups []v3.Member
	for _, m := range members {
		if m.UserPrincipalName != "" {
			memberUsers = append(memberUsers, m)
		} else if m.GroupPrincipalName != "" {
			memberGroups = append(memberGroups, m)
		}
	}
	sort.Slice(memberUsers, func(i, j int) bool { return memberUsers[i].UserPrincipalName < memberUsers[j].UserPrincipalName })
	sort.Slice(memberGroups, func(i, j int) bool { return memberGroups[i].GroupPrincipalName < memberGroups[j].GroupPrincipalName })
	memberUsers = append(memberUsers, memberGroups...)
	return memberUsers
}
