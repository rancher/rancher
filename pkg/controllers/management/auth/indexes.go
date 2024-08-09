package auth

import (
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func rbByOwner(rb *v1.RoleBinding) ([]string, error) {
	return getRBOwnerKey(rb), nil
}

// getRBOwnerKey returns the UIDs from the OwnerReferences of the provided RoleBindings.
// from the given RoleBinding object.
// It returns the list of OwnerReferences.
func getRBOwnerKey(rb *v1.RoleBinding) []string {
	var owners []string
	for _, o := range rb.OwnerReferences {
		owners = append(owners, string(o.UID))
	}
	return owners
}

// rbRoleSubjectKey returns a key identifier combining the given roleName and Subject.
func rbRoleSubjectKey(roleName string, subject v1.Subject) string {
	return roleName + "." + subject.Kind + "." + subject.Name
}

// rbRoleSubjectKeys returns a list of key identifiers.
// Similar to rbRoleSubjectKey.
func rbRoleSubjectKeys(roleName string, subjects []v1.Subject) []string {
	var keys []string
	for _, s := range subjects {
		keys = append(keys, rbRoleSubjectKey(roleName, s))
	}
	return keys
}

// indexByMembershipBindingOwner validate the object passed throught the arguments
// and return the list of keys whose value is the equal to "membership-binding-owner".
func indexByMembershipBindingOwner(obj interface{}) ([]string, error) {
	ro, ok := obj.(runtime.Object)
	if !ok {
		return []string{}, nil
	}

	accessor, err := meta.Accessor(ro)
	if err != nil {
		logrus.Warnf("[indexByMembershipBindingOwner] unexpected object type: %T, err: %v", obj, err.Error())
		return []string{}, nil
	}

	return rbObjectKeys(accessor)
}

// rbObjectKeys returns a formatted tuple of "namespace/key" from a generic object
// in case its labels values are equal to "membership-binding-owner".
func rbObjectKeys(metaObj metav1.Object) ([]string, error) {
	ns := metaObj.GetNamespace()
	var keys []string
	for k, v := range metaObj.GetLabels() {
		if v == MembershipBindingOwner {
			keys = append(keys, strings.Join([]string{ns, k}, "/"))
		}
	}
	return keys, nil
}

func rbByClusterRoleAndSubject(rb *v1.ClusterRoleBinding) ([]string, error) {
	var subjects []v1.Subject
	var roleName string

	roleName = rb.RoleRef.Name
	subjects = rb.Subjects
	return rbRoleSubjectKeys(roleName, subjects), nil
}

func rbByRoleAndSubject(rb *v1.RoleBinding) ([]string, error) {
	var subjects []v1.Subject
	var roleName string

	roleName = rb.RoleRef.Name
	subjects = rb.Subjects

	return rbRoleSubjectKeys(roleName, subjects), nil
}
