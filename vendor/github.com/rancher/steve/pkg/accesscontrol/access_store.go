package accesscontrol

import (
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/rbac/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

type AccessStore struct {
	users  *policyRuleIndex
	groups *policyRuleIndex
}

func NewAccessStore(rbac v1.Interface) *AccessStore {
	return &AccessStore{
		users:  newPolicyRuleIndex(true, rbac),
		groups: newPolicyRuleIndex(false, rbac),
	}
}

func (l *AccessStore) AccessFor(user user.Info) *AccessSet {
	result := l.users.get(user.GetName())
	for _, group := range user.GetGroups() {
		result.Merge(l.groups.get(group))
	}
	return result
}
