package globalnamespacerbac

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/rancher/rancher/pkg/namespace"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	rbacv1 "github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	"github.com/rancher/types/config"
	k8srbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	ownerAccess                     = "owner"
	memberAccess                    = "member"
	readOnlyAccess                  = "read-only"
	MultiClusterAppResource         = "multiclusterapps"
	MultiClusterAppRevisionResource = "multiclusterapprevisions"
	GlobalDNSResource               = "globaldnses"
	GlobalDNSProviderResource       = "globaldnsproviders"
	ClusterTemplateResource         = "clustertemplates"
	ClusterTemplateRevisionResource = "clustertemplaterevisions"
	CloudCredentialResource         = "secrets"
	CreatorIDAnn                    = "field.cattle.io/creatorId"
	RancherManagementAPIVersion     = "management.cattle.io"
)

var subjectWithAllUsers = k8srbacv1.Subject{
	Kind:     "Group",
	Name:     user.AllAuthenticated,
	APIGroup: rbacv1.GroupName,
}

func CreateRoleAndRoleBinding(resource, name, apiVersion, creatorID string, apiGroup []string, UID types.UID, members []v3.Member,
	mgmt *config.ManagementContext) error {
	/* Create 3 Roles containing the CRD, and the current CR in resourceNames list
	1. Role with owner verbs (Owner access, includes creator); name multiclusterapp.Name + "-ma" / (globalDNS.Name + "-ga")
	2. Role with "get", "list" "watch" verbs (ReadOnly access); name multiclusterapp.Name + "-mr" / (globalDNS.Name + "-gr")
	3. Role with "update" verb (Member access); name multiclusterapp.Name + "-mu" / (globalDNS.Name + "-gu")
	*/

	if _, err := createRole(resource, name, ownerAccess, apiVersion, apiGroup, UID, mgmt); err != nil {
		return err
	}

	// Create a roleBinding referring the role with everything access, and containing creator of the resouce, along with
	// any members that have everything access
	var ownerAccessSubjects, readOnlyAccessSubjects, memberAccessSubjects []k8srbacv1.Subject
	ownerAccessSubjects = append(ownerAccessSubjects, k8srbacv1.Subject{Kind: "User", Name: creatorID, APIGroup: rbacv1.GroupName})
	for _, m := range members {
		s, err := buildSubjectForMember(m, mgmt)
		if err != nil {
			return err
		}
		switch m.AccessType {
		case ownerAccess:
			ownerAccessSubjects = append(ownerAccessSubjects, s)
		case memberAccess:
			memberAccessSubjects = append(memberAccessSubjects, s)
		case readOnlyAccess:
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

	if err := createRoleBindingForMembers(resource, name, ownerAccess, apiVersion, UID, ownerAccessSubjects, mgmt); err != nil {
		return err
	}

	// Check if there are members with readonly or member(update) access; if found then create rolebindings for those
	if len(readOnlyAccessSubjects) > 0 {
		if _, err := createRole(resource, name, readOnlyAccess, apiVersion, apiGroup, UID, mgmt); err != nil {
			return err
		}
		if err := createRoleBindingForMembers(resource, name, readOnlyAccess, apiVersion, UID, readOnlyAccessSubjects, mgmt); err != nil {
			return err
		}
	}
	if len(memberAccessSubjects) > 0 {
		if _, err := createRole(resource, name, memberAccess, apiVersion, apiGroup, UID, mgmt); err != nil {
			return err
		}
		if err := createRoleBindingForMembers(resource, name, memberAccess, apiVersion, UID, memberAccessSubjects, mgmt); err != nil {
			return err
		}
	}
	return nil
}

func createRole(resourceType, resourceName, roleAccess, apiVersion string, apiGroups []string, resourceUID types.UID,
	mgmt *config.ManagementContext) (*k8srbacv1.Role, error) {
	roleName, verbs := getRoleNameAndVerbs(roleAccess, resourceName, resourceType)
	ownerReference := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       resourceType,
		Name:       resourceName,
		UID:        resourceUID,
	}
	newRole := &k8srbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            roleName,
			Namespace:       namespace.GlobalNamespace,
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
	role, err := mgmt.RBAC.Roles("").GetNamespaced(namespace.GlobalNamespace, roleName, metav1.GetOptions{})
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
		if !reflect.DeepEqual(newRole, role) {
			toUpdate := newRole.DeepCopy()
			updated, err := mgmt.RBAC.Roles("").Update(toUpdate)
			if err != nil {
				return updated, err
			}
		}
	}
	return role, nil
}

func createRoleBindingForMembers(resourceType, resourceName, roleAccess, apiVersion string, UID types.UID,
	subjects []k8srbacv1.Subject, mgmt *config.ManagementContext) error {
	roleName, _ := getRoleNameAndVerbs(roleAccess, resourceName, resourceType)
	// we can define the rolebinding first, since if it's not already present we can call create. And if it's present then we'll
	// still need to compare the current members' list
	sort.Slice(subjects, func(i, j int) bool { return subjects[i].Name < subjects[j].Name })
	return createRoleBinding(resourceType, resourceName, roleName, apiVersion, mgmt, UID, subjects)
}

func createRoleBinding(resourceType, resourceName, roleName, apiVersion string, mgmt *config.ManagementContext,
	resourceUID types.UID, subjects []k8srbacv1.Subject) error {
	ownerReference := metav1.OwnerReference{
		APIVersion: apiVersion,
		Kind:       resourceType,
		Name:       resourceName,
		UID:        resourceUID,
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
	roleBinding, err := mgmt.RBAC.RoleBindings("").GetNamespaced(namespace.GlobalNamespace, roleName, metav1.GetOptions{})
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
		if !reflect.DeepEqual(roleBinding, newRoleBinding) {
			toUpdate := newRoleBinding.DeepCopy()
			_, err := mgmt.RBAC.RoleBindings("").Update(toUpdate)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getRoleNameAndVerbs(roleAccess string, resourceName string, resourceType string) (string, []string) {
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
	}
	switch roleAccess {
	case ownerAccess:
		roleName = resourceName + "a"
		verbs = []string{"*"}
	case memberAccess:
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

	if name == "*" {
		//member.GroupPrincipalName = subjectWithAllUsers.Name
		return subjectWithAllUsers, nil
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
