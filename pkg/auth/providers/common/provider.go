package common

import (
	"net/http"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/auth/accessor"
)

const (
	// UserAttributePrincipalID is a key in the ExtraByProvider field of the
	// UserAttribute that holds principal IDs for a particular provider.
	UserAttributePrincipalID = "principalid"
	// UserAttributeUserName is a key in the ExtraByProvider field of the UserAttribute that holds usernames for a partucular provider.
	UserAttributeUserName = "username"
	// ExtraRequestTokenID is the key for the request token ID in the UserInfo's extra attributes.
	ExtraRequestTokenID = "requesttokenid"
	// ExtraRequestHost is the key for the request host name in the UserInfo's extra attributes.
	ExtraRequestHost = "requesthost"

	// UserPrincipalType is the user principal type across all providers.
	UserPrincipalType = "user"
	// GroupPrincipalType is the group principal type  across all providers.
	GroupPrincipalType = "group"
)

// AuthProvider allows to authenticate a user and search for user and group principals.
type AuthProvider interface {
	GetName() string
	AuthenticateUser(w http.ResponseWriter, r *http.Request, input any) (v3.Principal, []v3.Principal, string, error)
	SearchPrincipals(name, principalType string, myToken accessor.TokenAccessor) ([]v3.Principal, error)
	GetPrincipal(principalID string, token accessor.TokenAccessor) (v3.Principal, error)
	CustomizeSchema(schema *types.Schema)
	TransformToAuthProvider(authConfig map[string]any) (map[string]any, error)
	RefetchGroupPrincipals(principalID string, secret string) ([]v3.Principal, error)
	CanAccessWithGroupProviders(userPrincipalID string, groups []v3.Principal) (bool, error)
	// GetUserExtraAttributes retrieves the extra attributes from the specified principal.
	// Used during login, to create the login token.
	GetUserExtraAttributes(userPrincipal v3.Principal) map[string][]string
	IsDisabledProvider() (bool, error)

	// LogoutAll implements the "logout-all" action for the provider, if supported. If
	// "logout-all" is not supported do nothing and return nil.
	LogoutAll(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error

	// Logout implements a guard against invoking the "logout" action when "logout-all" is
	// forced. If "logout-all" is not supported by the provider do nothing and return nil.
	Logout(w http.ResponseWriter, r *http.Request, token accessor.TokenAccessor) error
}
