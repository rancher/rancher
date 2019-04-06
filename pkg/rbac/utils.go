package rbac

import (
	"github.com/pkg/errors"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func BuildSubjectFromRTB(target interface{}) (rbacv1.Subject, error) {
	var userName, groupPrincipalName, groupName, name, kind, namespace string
	if rtb, ok := target.(*v3.ProjectRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else if rtb, ok := target.(*v3.ClusterRoleTemplateBinding); ok {
		userName = rtb.UserName
		groupPrincipalName = rtb.GroupPrincipalName
		groupName = rtb.GroupName
	} else if sa, ok := target.(*corev1.ServiceAccount); ok {
		userName = sa.Name
		namespace = sa.Namespace
	} else {
		return rbacv1.Subject{}, errors.Errorf("unrecognized roleTemplateBinding type: %v", target)
	}

	if userName != "" {
		name = userName
		kind = "User"
	}

	if groupPrincipalName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", target)
		}
		name = groupPrincipalName
		kind = "Group"
	}

	if groupName != "" {
		if name != "" {
			return rbacv1.Subject{}, errors.Errorf("roletemplatebinding has more than one subject fields set: %v", target)
		}
		name = groupName
		kind = "Group"
	}

	if namespace != "" {
		kind = "ServiceAccount"
	}

	if name == "" {
		return rbacv1.Subject{}, errors.Errorf("roletemplatebinding doesn't have any subject fields set: %v", target)
	}

	return rbacv1.Subject{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}, nil
}

func BuildRule(resource string, verbs map[string]bool) rbacv1.PolicyRule {
	var vs []string
	for v := range verbs {
		vs = append(vs, v)
	}
	return rbacv1.PolicyRule{
		Resources: []string{resource},
		Verbs:     vs,
		APIGroups: []string{"*"},
	}
}
