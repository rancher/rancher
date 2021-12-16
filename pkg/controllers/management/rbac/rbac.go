package rbac

import (
	"fmt"
	"reflect"
	"sort"

	v32 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"

	rbacv1 "github.com/rancher/rancher/pkg/generated/norman/rbac.authorization.k8s.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	k8srbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	OwnerAccess                     = "owner"
	MemberAccess                    = "member"
	ReadOnlyAccess                  = "read-only"
	MultiClusterAppResource         = "multiclusterapps"
	MultiClusterAppRevisionResource = "multiclusterapprevisions"
	GlobalDNSResource               = "globaldnses"
	GlobalDNSProviderResource       = "globaldnsproviders"
	ClusterTemplateResource         = "clustertemplates"
	ClusterTemplateRevisionResource = "clustertemplaterevisions"
	CloudCredentialResource         = "secrets"
	CreatorIDAnn                    = "field.cattle.io/creatorId"
	RancherManagementAPIVersion     = "management.cattle.io/v3"
	RancherManagementAPIGroup       = "management.cattle.io"
	NodeTemplateResource            = "nodetemplates"
)

var subjectWithAllUsers = k8srbacv1.Subject{
	Kind:     "Group",
	Name:     user.AllAuthenticated,
	APIGroup: rbacv1.GroupName,
}

func CreateRoleAndRoleBinding(resource, kind, name, namespace, apiVersion, creatorID string, apiGroup []string, UID types.UID, members []v32.Member,
	mgmt *config.ManagementContext) error {
	/* Create 3 Roles containing the CRD, and the current CR in resourceNames list
	1. Role with owner verbs (Owner access, includes creator); name multiclusterapp.Name + "-ma" / (globalDNS.Name + "-ga")
	2. Role with "get", "list" "watch" verbs (ReadOnly access); name multiclusterapp.Name + "-mr" / (globalDNS.Name + "-gr")
	3. Role with "update" verb (Member access); name multiclusterapp.Name + "-mu" / (globalDNS.Name + "-gu")
	*/

	if _, err := createRole(resource, kind, name, namespace, OwnerAccess, apiVersion, apiGroup, UID, mgmt); err != nil {
		return err
	}

	// Create a roleBinding referring the role with everything access, and containing creator of the resource, along with
	// any members that have everything access
	var ownerAccessSubjects, readOnlyAccessSubjects, memberAccessSubjects []k8srbacv1.Subject
	ownerAccessSubjects = append(ownerAccessSubjects, k8srbacv1.Subject{Kind: "User", Name: creatorID, APIGroup: rbacv1.GroupName})
	for _, m := range members {
		s, err := buildSubjectForMember(m, mgmt)
		if err != nil {
			return err
		}
		switch m.AccessType {
		case OwnerAccess:
			ownerAccessSubjects = append(ownerAccessSubjects, s)
		case MemberAccess:
			memberAccessSubjects = append(memberAccessSubjects, s)
		case ReadOnlyAccess:
			readOnlyAccessSubjects = append(readOnlyAccessSubjects, s)
		default:
			if resource == GlobalDNSProviderResource || resource == GlobalDNSResource {
				// since these two resources only have one access type "owner" for their members
				ownerAccessSubjects = append(ownerAccessSubjects, s)
			} else {
				// for mcapp and cluster templates which can have other access types
				readOnlyAccessSubjects = append(readOnlyAccessSubjects, s)
			}
		}
	}

	// There will always be a role and rolebinding for owner access (admin)
	if err := createRoleBindingForMembers(resource, kind, name, namespace, OwnerAccess, apiVersion, UID, ownerAccessSubjects, mgmt); err != nil {
		return err
	}

	// Check if there are members with readonly or member(update) access; if found then create rolebindings for those
	if len(readOnlyAccessSubjects) > 0 {
		if _, err := createRole(resource, kind, name, namespace, ReadOnlyAccess, apiVersion, apiGroup, UID, mgmt); err != nil {
			return err
		}
		if err := createRoleBindingForMembers(resource, kind, name, namespace, ReadOnlyAccess, apiVersion, UID, readOnlyAccessSubjects, mgmt); err != nil {
			return err
		}
	} else {
		// check if rolebinding for read-only access exists, if it does then delete it, since there are no longer
		// any read-only members in the spec.
		roleName, _ := GetRoleNameAndVerbs(ReadOnlyAccess, name, resource)
		if err := deleteRoleAndRoleBinding(roleName, namespace, mgmt); err != nil {
			return err
		}
	}

	if len(memberAccessSubjects) > 0 {
		if _, err := createRole(resource, kind, name, namespace, MemberAccess, apiVersion, apiGroup, UID, mgmt); err != nil {
			return err
		}
		if err := createRoleBindingForMembers(resource, kind, name, namespace, MemberAccess, apiVersion, UID, memberAccessSubjects, mgmt); err != nil {
			return err
		}
	} else {
		// check if rolebinding for member access (update access type) exists, if it does then delete it,
		// since there are no longer any members with update access in the spec.
		roleName, _ := GetRoleNameAndVerbs(MemberAccess, name, resource)
		return deleteRoleAndRoleBinding(roleName, namespace, mgmt)
	}
	return nil
}

func createRole(resourceType, resourceKind, resourceName, namespace, roleAccess, apiVersion string, apiGroups []string, resourceUID types.UID,
	mgmt *config.ManagementContext) (*k8srbacv1.Role, error) {
	roleName, verbs := GetRoleNameAndVerbs(roleAccess, resourceName, resourceType)
	ownerReference := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       resourceKind,
		Name:       resourceName,
		UID:        resourceUID,
	}
	newRole := &k8srbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            roleName,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Rules: []k8srbacv1.PolicyRule{
			{
				APIGroups:     apiGroups,
				Resources:     []string{resourceType},
				ResourceNames: []string{resourceName},
				Verbs:         verbs,
			},
		},
	}
	role, err := mgmt.RBAC.Roles("").Controller().Lister().Get(namespace, roleName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			role, err = mgmt.RBAC.Roles("").Create(newRole)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if role != nil {
		if !reflect.DeepEqual(newRole.Rules, role.Rules) {
			toUpdate := newRole.DeepCopy()
			updated, err := mgmt.RBAC.Roles("").Update(toUpdate)
			if err != nil {
				return updated, err
			}
		}
	}
	return role, nil
}

func createRoleBindingForMembers(resourceType, resourceKind, resourceName, namespace, roleAccess, apiVersion string, UID types.UID,
	subjects []k8srbacv1.Subject, mgmt *config.ManagementContext) error {
	roleName, _ := GetRoleNameAndVerbs(roleAccess, resourceName, resourceType)
	// we can define the rolebinding first, since if it's not already present we can call create. And if it's present then we'll
	// still need to compare the current members' list
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })
	return createRoleBinding(resourceKind, resourceName, namespace, roleName, apiVersion, mgmt, UID, subjects)
}

func createRoleBinding(resourceKind, resourceName, namespace, roleName, apiVersion string, mgmt *config.ManagementContext,
	resourceUID types.UID, subjects []k8srbacv1.Subject) error {
	ownerReference := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       resourceKind,
		Name:       resourceName,
		UID:        resourceUID,
	}
	newRoleBinding := &k8srbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            roleName,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		RoleRef: k8srbacv1.RoleRef{
			Name: roleName,
			Kind: "Role",
		},
		Subjects: subjects,
	}

	roleBinding, err := mgmt.RBAC.RoleBindings("").Controller().Lister().Get(namespace, roleName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_, err = mgmt.RBAC.RoleBindings("").Create(newRoleBinding)
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return err
			}
		} else {
			return err
		}
	} else if roleBinding != nil {
		if !reflect.DeepEqual(roleBinding.Subjects, newRoleBinding.Subjects) {
			toUpdate := newRoleBinding.DeepCopy()
			_, err := mgmt.RBAC.RoleBindings("").Update(toUpdate)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func GetRoleNameAndVerbs(roleAccess string, resourceName string, resourceType string) (string, []string) {
	var roleName string
	var verbs []string

	switch resourceType {
	case MultiClusterAppResource:
		resourceName += "-m-"
	case MultiClusterAppRevisionResource:
		resourceName += "-mr-"
	case GlobalDNSResource:
		resourceName += "-g-"
	case GlobalDNSProviderResource:
		resourceName += "-gp-"
	case ClusterTemplateResource:
		resourceName += "-ct-"
	case ClusterTemplateRevisionResource:
		resourceName += "-ctr-"
	case NodeTemplateResource:
		resourceName += "-nt-"
	default:
		resourceName += "-" + resourceType + "-"
	}
	switch roleAccess {
	case OwnerAccess:
		roleName = resourceName + "a"
		verbs = []string{"*"}
	case MemberAccess:
		roleName = resourceName + "u"
		verbs = []string{"update", "get", "list", "watch"}
	case ReadOnlyAccess:
		roleName = resourceName + "r"
		verbs = []string{"get", "list", "watch"}
	}

	return roleName, verbs
}

func deleteRoleAndRoleBinding(roleName, namespace string, mgmt *config.ManagementContext) error {
	err := mgmt.RBAC.Roles("").DeleteNamespaced(namespace, roleName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
		return err
	}
	err = mgmt.RBAC.RoleBindings("").DeleteNamespaced(namespace, roleName, &metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) && !apierrors.IsGone(err) {
		return err
	}
	return nil
}

func buildSubjectForMember(member v32.Member, managementContext *config.ManagementContext) (k8srbacv1.Subject, error) {
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

	if name == "*" {
		// member.GroupPrincipalName = subjectWithAllUsers.Name
		return subjectWithAllUsers, nil
	}

	return k8srbacv1.Subject{
		Kind:     kind,
		Name:     name,
		APIGroup: rbacv1.GroupName,
	}, nil
}

func checkAndSetUserFields(m v32.Member, managementContext *config.ManagementContext) (v32.Member, error) {
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
