package auth

import (
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	meta2 "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func rbByOwner(rb *v1.RoleBinding) ([]string, error) {
	return getRBOwnerKey(rb), nil
}

func getRBOwnerKey(rb *v1.RoleBinding) []string {
	var owners []string
	for _, o := range rb.OwnerReferences {
		owners = append(owners, string(o.UID))
	}
	return owners
}

func rbRoleSubjectKey(roleName string, subject v1.Subject) string {
	return roleName + "." + subject.Kind + "." + subject.Name
}

func rbRoleSubjectKeys(roleName string, subjects []v1.Subject) []string {
	var keys []string
	for _, s := range subjects {
		keys = append(keys, rbRoleSubjectKey(roleName, s))
	}
	return keys
}

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
