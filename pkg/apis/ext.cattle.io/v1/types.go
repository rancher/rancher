// +kubebuilder:skip
package v1

import (
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// copied from package "pkg/auth/providers/common" (provider.go).
	// BEWARE the local copies are necessary because import of the package
	// triggers CI failure around modules and dependencies, crewjam/saml in
	// particular.
	UserAttributePrincipalID = "principalid"
	UserAttributeUserName    = "username"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Token is the main extension Token structure
type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the spec of RancherToken
	Spec TokenSpec `json:"spec"`
	// +optional
	Status TokenStatus `json:"status"`
}

// TokenSpec contains the user-specifiable parts of the Token
type TokenSpec struct {
	// UserID is the user id
	UserID string `json:"userID"`
	// Human readable description.
	// +optional
	Description string `json:"description, omitempty"`
	// ClusterName is the cluster that the token is scoped to.
	// When empty, the default, the token can be used for all clusters.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// TTL is the time-to-live of the token, in milliseconds.
	// The default of 0 is treated as 30 days.
	// +optional
	TTL int64 `json:"ttl"`
	// Enabled indicates an active token.
	// The `null` state maps to `true`, making that the default.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Kind describes the nature of the token. The value "session"
	// indicates a login/session token. Any other value, including
	// the empty string, which is the default, stands for some kind
	// of derived token.
	// +optional
	Kind string `json:"kind"`
}

// TokenStatus contains the data derived from the specification or otherwise generated.
type TokenStatus struct {
	// Note: All fields in this structure are ignored by Update operations.
	// They cannot be changed and attempts to do so are ignored without
	// warning, nor error. They represent system data a user is allowed to
	// see, but not modify.

	// TokenValue is the access key. Shown only on token creation. Not saved.
	TokenValue string `json:"tokenValue,omitempty"`
	// TokenHash is the hash of the value. Only thing saved.
	TokenHash string `json:"tokenHash,omitempty"`

	// Current is a boolean flag. It is set to true for a token returned by
	// a get, list, or watch request, when the token is the token which
	// authenticated that request. All other tokens returned by the request,
	// if any, will have this flag set to false.
	Current bool `json:"current"`

	// Time derived data. These fields are not stored in the backing secret.
	// Both values can be trivially computed from the secret's/token's
	// creation time, the time to live, and the current time.

	// Expired is a boolean flag, derived from creation time and
	// time-to-live. True indicates that the token has exceeded its time to
	// live, as counted from creation.
	Expired bool `json:"expired"`
	// ExpiresAt is creation time + time-to-live, i.e. the time when the
	// token expires.  This is set to the empty string if the token does not
	// expire at all.
	ExpiresAt string `json:"expiresAt"`

	// User derived data. This information is complex/expensive to
	// determine. As such this is stored in the backing secret to avoid
	// recomputing it whenever the token is retrieved.

	// AuthProvider names the auth provider managing the user. This
	// information is retrieved from the UserAttribute resource referenced
	// by `Spec.UserID`.
	AuthProvider string `json:"authProvider"`

	// DisplayName is the display name of the User referenced by
	// `Spec.UserID`. Stored as it is one of the pieces required to to
	// internally assemble a v3.Principal structure for the token.
	DisplayName string `json:displayName`

	// LoginName is the name of the User referenced by `Spec.UserID`. Stored
	// as it is one of the pieces required to to internally assemble a
	// v3.Principal structure for the token.
	LoginName string `json:loginName`

	// PrincipalID is retrieved from the UserAttribute resource referenced by
	// `Spec.UserID`. It is the first principal id found for the auth
	// provider. Stored as it is one of the pieces required to internally
	// assemble a v3.Principal structure for the token.
	PrincipalID string `json:principalID`

	// GroupPrincipals holds detailed group information
	// This is not supported here.
	// The primary location for this information are the UserAttribute resources.
	// The norman tokens maintain this only as legacy.
	// The ext tokens here shed this legacy.

	// ProviderInfo provides provider-specific information.
	// This is not supported here.
	// The actual primary storage for this is a regular k8s Secret associated with the User.
	// The norman tokens maintains this only as legacy for the `access_token` of OIDC-based auth providers.
	// The ext tokens here shed this legacy.

	// LastUpdateTime provides the time of last change to the token
	LastUpdateTime string `json:"lastUpdateTime"`

	// LastUsedAt provides the last time the token was used in a request, at second granularity.
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`

	// FUTURE ((USER ACTIVITY)) IdleTimeout provides the timeout used by the user activity monitoring.
	// IdleTimeout ... `json:"idleTimeout,omitempty"`
}

// Implement the TokenAccessor interface

func (t *Token) GetName() string {
	return t.ObjectMeta.Name
}

func (t *Token) GetIsEnabled() bool {
	return t.Spec.Enabled == nil || *t.Spec.Enabled
}

func (t *Token) GetIsDerived() bool {
	// session is the kind of login tokens, the only kind of non-derived tokens.
	return t.Spec.Kind != "session"
}

func (t *Token) GetUserID() string {
	return t.Spec.UserID
}

func (t *Token) ObjClusterName() string {
	return t.Spec.ClusterName
}

func (t *Token) GetAuthProvider() string {
	return t.Status.AuthProvider
}

func (t *Token) GetUserPrincipal() apiv3.Principal {
	return apiv3.Principal{
		ObjectMeta: metav1.ObjectMeta{
			Name: t.Status.PrincipalID,
		},
		DisplayName:   t.Status.DisplayName,
		LoginName:     t.Status.LoginName,
		Provider:      t.Status.AuthProvider,
		PrincipalType: "user",
		ExtraInfo: map[string]string{
			UserAttributePrincipalID: t.Status.PrincipalID,
			UserAttributeUserName:    t.Status.LoginName,
		},
	}
}

func (t *Token) GetGroupPrincipals() []apiv3.Principal {
	// Not supported. Legacy in Norman tokens.
	return []apiv3.Principal{}
}

func (t *Token) GetProviderInfo() map[string]string {
	// Not supported. Legacy in Norman tokens.
	return map[string]string{}
}

func (t *Token) GetLastUsedAt() *metav1.Time {
	return t.Status.LastUsedAt
}
