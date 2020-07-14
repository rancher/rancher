package accesscontrol

import (
	"hash"
	"sort"

	v1 "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	rbacGroup = "rbac.authorization.k8s.io"
	All       = "*"
)

type policyRuleIndex struct {
	crCache             v1.ClusterRoleCache
	rCache              v1.RoleCache
	crbCache            v1.ClusterRoleBindingCache
	rbCache             v1.RoleBindingCache
	revisions           *roleRevisionIndex
	kind                string
	roleIndexKey        string
	clusterRoleIndexKey string
}

func newPolicyRuleIndex(user bool, revisions *roleRevisionIndex, rbac v1.Interface) *policyRuleIndex {
	key := "Group"
	if user {
		key = "User"
	}
	pi := &policyRuleIndex{
		kind:                key,
		crCache:             rbac.ClusterRole().Cache(),
		rCache:              rbac.Role().Cache(),
		crbCache:            rbac.ClusterRoleBinding().Cache(),
		rbCache:             rbac.RoleBinding().Cache(),
		clusterRoleIndexKey: "crb" + key,
		roleIndexKey:        "rb" + key,
		revisions:           revisions,
	}

	pi.crbCache.AddIndexer(pi.clusterRoleIndexKey, pi.clusterRoleBindingBySubjectIndexer)
	pi.rbCache.AddIndexer(pi.roleIndexKey, pi.roleBindingBySubject)

	return pi
}

func (p *policyRuleIndex) clusterRoleBindingBySubjectIndexer(crb *rbacv1.ClusterRoleBinding) (result []string, err error) {
	for _, subject := range crb.Subjects {
		if subject.APIGroup == rbacGroup && subject.Kind == p.kind && crb.RoleRef.Kind == "ClusterRole" {
			result = append(result, subject.Name)
		}
	}
	return
}

func (p *policyRuleIndex) roleBindingBySubject(rb *rbacv1.RoleBinding) (result []string, err error) {
	for _, subject := range rb.Subjects {
		if subject.APIGroup == rbacGroup && subject.Kind == p.kind {
			result = append(result, subject.Name)
		}
	}
	return
}

var null = []byte{'\x00'}

func (p *policyRuleIndex) addRolesToHash(digest hash.Hash, subjectName string) {
	for _, crb := range p.getClusterRoleBindings(subjectName) {
		digest.Write([]byte(crb.RoleRef.Name))
		digest.Write([]byte(p.revisions.roleRevision("", crb.RoleRef.Name)))
		digest.Write(null)
	}

	for _, rb := range p.getRoleBindings(subjectName) {
		switch rb.RoleRef.Kind {
		case "Role":
			digest.Write([]byte(rb.RoleRef.Name))
			digest.Write([]byte(rb.Namespace))
			digest.Write([]byte(p.revisions.roleRevision(rb.Namespace, rb.RoleRef.Name)))
			digest.Write(null)
		case "ClusterRole":
			digest.Write([]byte(rb.RoleRef.Name))
			digest.Write([]byte(rb.Namespace))
			digest.Write([]byte(p.revisions.roleRevision(rb.Namespace, rb.RoleRef.Name)))
			digest.Write(null)
		}
	}
}

func (p *policyRuleIndex) get(subjectName string) *AccessSet {
	result := &AccessSet{}

	for _, binding := range p.getRoleBindings(subjectName) {
		p.addAccess(result, binding.Namespace, binding.RoleRef)
	}

	for _, binding := range p.getClusterRoleBindings(subjectName) {
		p.addAccess(result, All, binding.RoleRef)
	}

	return result
}

func (p *policyRuleIndex) addAccess(accessSet *AccessSet, namespace string, roleRef rbacv1.RoleRef) {
	for _, rule := range p.getRules(namespace, roleRef) {
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				names := rule.ResourceNames
				if len(names) == 0 {
					names = []string{All}
				}
				for _, resourceName := range names {
					for _, verb := range rule.Verbs {
						accessSet.Add(verb,
							schema.GroupResource{
								Group:    group,
								Resource: resource,
							}, Access{
								Namespace:    namespace,
								ResourceName: resourceName,
							})
					}
				}
			}
		}
	}
}

func (p *policyRuleIndex) getRules(namespace string, roleRef rbacv1.RoleRef) []rbacv1.PolicyRule {
	switch roleRef.Kind {
	case "ClusterRole":
		role, err := p.crCache.Get(roleRef.Name)
		if err != nil {
			return nil
		}
		return role.Rules
	case "Role":
		role, err := p.rCache.Get(namespace, roleRef.Name)
		if err != nil {
			return nil
		}
		return role.Rules
	}

	return nil
}

func (p *policyRuleIndex) getClusterRoleBindings(subjectName string) []*rbacv1.ClusterRoleBinding {
	result, err := p.crbCache.GetByIndex(p.clusterRoleIndexKey, subjectName)
	if err != nil {
		return nil
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (p *policyRuleIndex) getRoleBindings(subjectName string) []*rbacv1.RoleBinding {
	result, err := p.rbCache.GetByIndex(p.roleIndexKey, subjectName)
	if err != nil {
		return nil
	}
	sort.Slice(result, func(i, j int) bool {
		return string(result[i].UID) < string(result[j].UID)
	})
	return result
}
