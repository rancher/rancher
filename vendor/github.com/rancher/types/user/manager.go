package user

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/types/apis/management.cattle.io/v3"
)

type Manager interface {
	SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error)
	GetUser(apiContext *types.APIContext) string
	EnsureUser(principalName, displayName string) (*v3.User, error)
	CheckAccess(accessMode string, allowedPrincipalIDs []string, user v3.Principal, groups []v3.Principal) (bool, error)
}
