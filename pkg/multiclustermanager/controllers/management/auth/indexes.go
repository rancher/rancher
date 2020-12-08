package auth

import (
	v1 "k8s.io/api/rbac/v1"
)

func rbRoleSubjectKey(roleName string, subject v1.Subject) string {
	return roleName + "." + subject.Kind + "." + subject.Name
}
