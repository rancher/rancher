package rbac

import (
	"strings"

	"github.com/rancher/norman/types/slice"
	"github.com/rancher/types/apis/rbac.authorization.k8s.io/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	rbacGroup = "rbac.authorization.k8s.io"
)

func newIndexes(client v1.Interface) (user *permissionIndex, group *permissionIndex) {
	client.ClusterRoleBindings("").Controller().Informer().
		AddIndexers(cache.Indexers{
			"crbUser": func(obj interface{}) ([]string, error) {
				return clusterRoleBindingBySubject("User", obj)
			},
			"crbGroup": func(obj interface{}) ([]string, error) {
				return clusterRoleBindingBySubject("Group", obj)
			},
		})

	client.RoleBindings("").Controller().Informer().
		AddIndexers(cache.Indexers{
			"rbUser": func(obj interface{}) ([]string, error) {
				return roleBindingBySubject("User", obj)
			},
			"rbGroup": func(obj interface{}) ([]string, error) {
				return roleBindingBySubject("Group", obj)
			},
		})

	user = &permissionIndex{
		clusterRoleLister:   client.ClusterRoles("").Controller().Lister(),
		roleLister:          client.Roles("").Controller().Lister(),
		crbIndexer:          client.ClusterRoleBindings("").Controller().Informer().GetIndexer(),
		rbIndexer:           client.RoleBindings("").Controller().Informer().GetIndexer(),
		roleIndexKey:        "rbUser",
		clusterRoleIndexKey: "crbUser",
	}

	group = &permissionIndex{
		clusterRoleLister:   client.ClusterRoles("").Controller().Lister(),
		roleLister:          client.Roles("").Controller().Lister(),
		crbIndexer:          client.ClusterRoleBindings("").Controller().Informer().GetIndexer(),
		rbIndexer:           client.RoleBindings("").Controller().Informer().GetIndexer(),
		roleIndexKey:        "rbGroup",
		clusterRoleIndexKey: "crbGroup",
	}

	return
}

func clusterRoleBindingBySubject(kind string, obj interface{}) ([]string, error) {
	var result []string
	crb := obj.(*rbacv1.ClusterRoleBinding)
	for _, subject := range crb.Subjects {
		if subject.Kind == kind {
			result = append(result, subject.Name)
		}
	}
	return result, nil
}

func roleBindingBySubject(kind string, obj interface{}) ([]string, error) {
	var result []string
	crb := obj.(*rbacv1.RoleBinding)
	for _, subject := range crb.Subjects {
		if subject.Kind == kind {
			result = append(result, subject.Name)
		}
	}
	return result, nil
}

type permissionIndex struct {
	clusterRoleLister   v1.ClusterRoleLister
	roleLister          v1.RoleLister
	crbIndexer          cache.Indexer
	rbIndexer           cache.Indexer
	roleIndexKey        string
	clusterRoleIndexKey string
}

func (p *permissionIndex) get(subjectName, apiGroup, resource, verb string) []ListPermission {
	var result []ListPermission

	for _, binding := range p.getRoleBindings(subjectName) {
		if binding.RoleRef.APIGroup != rbacGroup {
			continue
		}

		result = p.filterPermissions(result, binding.Namespace, binding.RoleRef.Kind, binding.RoleRef.Name, apiGroup, resource, verb)
	}

	for _, binding := range p.getClusterRoleBindings(subjectName) {
		if binding.RoleRef.APIGroup != rbacGroup {
			continue
		}
		result = p.filterPermissions(result, "*", binding.RoleRef.Kind, binding.RoleRef.Name, apiGroup, resource, verb)
	}

	return result
}

func (p *permissionIndex) filterPermissions(result []ListPermission, namespace, kind, name, apiGroup, resource, verb string) []ListPermission {
	nsForResourceNameGets := namespace
	if namespace == "*" {
		nsForResourceNameGets = ""
	}

	for _, rule := range p.getRules(namespace, kind, name) {
		if !matches(rule.APIGroups, apiGroup) || !matches(rule.Resources, resource) {
			continue
		}

		if len(rule.ResourceNames) > 0 {
			// special case: if verb is list and this rule has resourceNames, check for the verb get instead of list
			v := verb
			if verb == "list" {
				v = "get"
			}
			if slice.ContainsString(rule.Verbs, "*") || slice.ContainsString(rule.Verbs, v) {
				for _, resourceName := range rule.ResourceNames {
					result = append(result, ListPermission{
						Namespace: nsForResourceNameGets,
						Name:      resourceName,
					})
				}
			}
			continue
		}

		if slice.ContainsString(rule.Verbs, "*") || slice.ContainsString(rule.Verbs, verb) {
			result = append(result, ListPermission{
				Namespace: namespace,
				Name:      "*",
			})
		}
	}

	return result
}

func (p *permissionIndex) getClusterRoleBindings(subjectName string) []*rbacv1.ClusterRoleBinding {
	var result []*rbacv1.ClusterRoleBinding

	objs, err := p.crbIndexer.ByIndex(p.clusterRoleIndexKey, subjectName)
	if err != nil {
		return result
	}

	for _, obj := range objs {
		result = append(result, obj.(*rbacv1.ClusterRoleBinding))
	}

	return result
}

func (p *permissionIndex) getRoleBindings(subjectName string) []*rbacv1.RoleBinding {
	var result []*rbacv1.RoleBinding

	objs, err := p.rbIndexer.ByIndex(p.roleIndexKey, subjectName)
	if err != nil {
		return result
	}

	for _, obj := range objs {
		result = append(result, obj.(*rbacv1.RoleBinding))
	}

	return result
}

func (p *permissionIndex) getRules(namespace, kind, name string) []rbacv1.PolicyRule {
	switch kind {
	case "ClusterRole":
		role, err := p.clusterRoleLister.Get("", name)
		if err != nil {
			return nil
		}
		return role.Rules
	case "Role":
		role, err := p.roleLister.Get(namespace, name)
		if err != nil {
			return nil
		}
		return role.Rules
	}

	return nil
}

func matches(parts []string, val string) bool {
	for _, value := range parts {
		if value == "*" {
			return true
		}
		if strings.EqualFold(value, val) {
			return true
		}
	}
	return false
}
