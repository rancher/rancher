package user

import (
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	apitypes "k8s.io/apimachinery/pkg/types"
)

type TokenInput struct {
	TokenName     string
	Description   string
	Kind          string
	UserName      string
	AuthProvider  string
	TTL           *int64
	Randomize     bool
	UserPrincipal v3.Principal
}

type Manager interface {
	SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error)
	GetUser(apiContext *types.APIContext) string
	EnsureToken(input TokenInput) (string, error)
	EnsureClusterToken(clusterName string, input TokenInput) (string, error)
	DeleteToken(tokenName string) error
	EnsureUser(principalName, displayName string) (*v3.User, error)
	CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error)
	SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error)
	CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error
	GetUserByPrincipalID(principalName string) (*v3.User, error)
	GetKubeconfigToken(clusterName, tokenName, description, kind, userName string, userPrincipal v3.Principal) (*v3.Token, string, error)
}
