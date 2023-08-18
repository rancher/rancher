package auth

import (
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

// rbByOwner returns the set of owner UIDs associated with a RoleBinding and a nil error.
func rbByOwner(rb *v1.RoleBinding) ([]string, error) {
	return getRBOwnerKey(rb), nil
}

// getRBOwnerKey returns the set of owner UIDs associated with a RoleBinding.
func getRBOwnerKey(rb *v1.RoleBinding) []string {
	var owners []string
	for _, o := range rb.OwnerReferences {
		owners = append(owners, string(o.UID))
	}
	return owners
}

// rbRoleSubjectKey returns a RoleBinding subject key with unique values.
func rbRoleSubjectKey(roleName string, subject v1.Subject) string {
	return roleName + "." + subject.Kind + "." + subject.Name
}

// rbRoleSubjectKeys returns the set of RoleBinding subject keys for a role and subject.
func rbRoleSubjectKeys(roleName string, subjects []v1.Subject) []string {
	var keys []string
	for _, s := range subjects {
		keys = append(keys, rbRoleSubjectKey(roleName, s))
	}
	return keys
}

// indexByMembershipBindingOwner returns an index list of {namespace, key} where the value in {key, value} is a
// MembershipBindingOwner.
// todo: This parses an API object and displays all the owner keys for the namespace the k8s object is in?
func indexByMembershipBindingOwner(obj interface{}) ([]string, error) {
	ro, ok := obj.(runtime.Object)
	if !ok {
		return []string{}, nil
	}

	meta, err := meta2.Accessor(ro)
	if err != nil {
		logrus.Warnf("[indexByMembershipBindingOwner] unexpected object type: %T, err: %v", obj, err.Error())
		return []string{}, nil
	}

	ns := meta.GetNamespace()
	var keys []string
	for k, v := range meta.GetLabels() {
		if v == MembershipBindingOwner {
			keys = append(keys, strings.Join([]string{ns, k}, "/"))
		}
	}

	return keys, nil
}

// rbByClusterRoleAndSubject returns the set of RoleBinding subject keys for a ClusterRoleBinding.
func rbByClusterRoleAndSubject(rb *v1.ClusterRoleBinding) ([]string, error) {
	var subjects []v1.Subject
	var roleName string

	roleName = rb.RoleRef.Name
	subjects = rb.Subjects
	return rbRoleSubjectKeys(roleName, subjects), nil
}

// rbByRoleAndSubject returns the set of RoleBinding subject keys for a RoleBinding.
func rbByRoleAndSubject(rb *v1.RoleBinding) ([]string, error) {
	var subjects []v1.Subject
	var roleName string

	roleName = rb.RoleRef.Name
	subjects = rb.Subjects

	return rbRoleSubjectKeys(roleName, subjects), nil
}
