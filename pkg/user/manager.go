package user

import (
	"time"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
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
	Labels        map[string]string
}

type Manager interface {
	GetUser(apiContext *types.APIContext) string
	EnsureUser(principalName, displayName string) (*v3.User, error)
	CheckAccess(accessMode string, allowedPrincipalIDs []string, userPrincipalID string, groups []v3.Principal) (bool, error)
	SetPrincipalOnCurrentUserByUserID(userID string, principal v3.Principal) (*v3.User, error)
	SetPrincipalOnCurrentUser(apiContext *types.APIContext, principal v3.Principal) (*v3.User, error)
	CreateNewUserClusterRoleBinding(userName string, userUID apitypes.UID) error
	GetUserByPrincipalID(principalName string) (*v3.User, error)
	GetGroupsForTokenAuthProvider(token accessor.TokenAccessor) []v3.Principal
	EnsureAndGetUserAttribute(userID string) (*v3.UserAttribute, bool, error)
	IsMemberOf(token accessor.TokenAccessor, group v3.Principal) bool
	UserAttributeCreateOrUpdate(userID, provider string, groupPrincipals []v3.Principal, userExtraInfo map[string][]string, loginTime ...time.Time) error
}
