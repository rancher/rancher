package accessor

import (
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TokenAccessor is an interface hiding the details of the different Token
// structures from the parts of Rancher having to work with both. The structures
// currently implementing this interface are Norman in file
// `pkg/apis/management.cattle.io/v3/authn_types.go`, and imperative in file
// `pkg/apis/ext.cattle.io/v1/types.go`
type TokenAccessor interface {
	// GetName returns the resource name of the token.
	GetName() string
	// GetIsEnabled returns a boolean flag indicating if the token is
	// enabled or not.
	GetIsEnabled() bool
	// GetIsDerived returns a boolean flag indicating if the token is a
	// derived (non-session) token, or not (session token).
	GetIsDerived() bool
	// GetAuthProvider returns the name of the auth provider controlling the
	// token. This is separate from `GetUserPrincipal().Provider` (A)
	// because v3 Tokens have a separate field, which is used everywhere,
	// and we cannot be sure that it is always in sync with (A).
	GetAuthProvider() string
	// GetUserID returns the id of the user owning the token.
	GetUserID() string
	// GetProviderInfo returns a map of provider-specific information.
	GetProviderInfo() map[string]string
	// ObjClusterName returns the name of the cluter the token is restricted
	// to. An empty string indicates "no restrictions". This method does not
	// use the `Get` prefix because it existed before and changing it was
	// deemed to risky.
	ObjClusterName() string
	// GetUserPrincipal returns the data of the controlling principal
	GetUserPrincipal() v3.Principal
	// GetGroupPrincipals returns a slice of group principal information.
	GetGroupPrincipals() []v3.Principal
	// GetLastUsedAt returns the time of the token's last use.
	GetLastUsedAt() *metav1.Time
	// GetLastActivitySeen returns the time of the last recorded activity for the token
	GetLastActivitySeen() *metav1.Time
	// GetCreationTime returns the creation time of the token
	GetCreationTime() metav1.Time
	// GetExpiresAt returns the Expiration time of the token, as string
	GetExpiresAt() string
}
