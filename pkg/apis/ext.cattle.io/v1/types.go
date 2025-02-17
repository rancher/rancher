// +kubebuilder:skip
package v1

import (
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Token instances are used to authenticate requests made against the Rancher backend.
type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec contains the user-accessible configuration of the token.
	Spec TokenSpec `json:"spec"`
	// Status contains system information about the token.
	Status TokenStatus `json:"status"`
}

// TokenSpec contains the user-specifiable parts of the Token.
type TokenSpec struct {
	// UserID is the kube resource id of the user owning the token. By
	// default that is the user who owned the token making the request
	// creating this token. Currently this default is enforced, i.e. using a
	// different user is rejected as forbidden.
	// +optional
	UserID string `json:"userID,omitempty"`
	// PrincipalID is the id of the user in the auth provider used. By
	// default that is the principal who owns the token making the request
	// creating this token. Currently this default is enforced, i.e. using a
	// different principle is rejected as forbidden.
	// +optional
	PrincipalID string `json:principalID,omitempty`
	// Human readable description.
	// +optional
	Description string `json:"description, omitempty"`
	// TTL is the time-to-live of the token, in milliseconds.
	// The default, 30 days, is indicated by the value `0`.
	// +optional
	TTL int64 `json:"ttl"`
	// Enabled indicates an active token. The default (`null`) indicates an
	// enabled token.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// Kind describes the nature of the token. The value "session" indicates
	// a login/session token. Any other value, including the empty string,
	// which is the default, stands for some kind of derived token.
	// +optional
	Kind string `json:"kind"`
}

// TokenStatus contains system information about the Token.
type TokenStatus struct {
	// TokenValue is the access key. It is shown only on token creation and not saved.
	TokenValue string `json:"tokenValue,omitempty"`
	// TokenHash is the hash of the TokenValue.
	TokenHash string `json:"tokenHash,omitempty"`
	// Current is a boolean flag. It is set to true for a token returned by
	// a get, list, or watch request, when the token is the token which
	// authenticated that request. All other tokens returned by the request,
	// if any, will have this flag set to false.
	Current bool `json:"current"`
	// Expired is a boolean flag. True indicates that the token has exceeded
	// its time to live, as counted from the Token's creation.
	Expired bool `json:"expired"`
	// ExpiresAt provides the time when the token expires. This field is set
	// to the empty string if the token does not expire at all.
	ExpiresAt string `json:"expiresAt"`
	// AuthProvider names the auth provider managing the user owning the token.
	AuthProvider string `json:"authProvider"`
	// DisplayName is the display name of the User owning the token.
	DisplayName string `json:displayName`
	// UserName is the regular name of the User owning the token.
	UserName string `json:userName`
	// LastUpdateTime provides the time of the last change to the token
	LastUpdateTime string `json:"lastUpdateTime"`
	// LastUsedAt provides the last time the token was used in a request, at
	// second granularity.
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`
}

// FUTURE ((USER ACTIVITY)) Add above -- IdleTimeout provides the timeout used by the user activity monitoring.
// IdleTimeout ... `json:"idleTimeout,omitempty"`

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
	return ""
}

func (t *Token) GetAuthProvider() string {
	return t.Status.AuthProvider
}

func (t *Token) GetUserPrincipalID() string {
	return t.Spec.PrincipalID
}

func (t *Token) GetUserPrincipalType() string {
	return "user"
}

func (t *Token) GetUserDisplayName() string {
	return t.Status.DisplayName
}

func (t *Token) GetUserName() string {
	return t.Status.UserName
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
