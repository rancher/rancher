// +kubebuilder:skip
package v1

import (
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserActivity keeps tracks user activity in the UI.
// If the user doesn't perform certain actions for a while e.g. cursor moved, key pressed, etc.,
// this will lead to the user being logged out of the session.
type UserActivity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Status is the most recently observed status of the UserActivity.
	Status UserActivityStatus `json:"status"`
}

// UserActivityStatus defines the most recently observed status of the UserActivity.
type UserActivityStatus struct {
	// ExpiresAt is the timestamp at which the user's session expires if it stays idle, invalidating the corresponding session token.
	// It is calculated by adding the duration specified in the auth-user-session-idle-ttl-minutes setting to the time of the request.
	// +optional
	ExpiresAt string `json:"expiresAt"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Token is used to authenticate requests to Rancher.
type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired state of the Token.
	Spec TokenSpec `json:"spec"`
	// Status is the most recently observed status of the Token.
	Status TokenStatus `json:"status"`
}

// TokenSpec defines the desired state of the Token.
type TokenSpec struct {
	// UserID is the kube resource id of the user owning the token. By
	// default that is the user who owned the token making the request
	// creating this token. Currently this default is enforced, i.e. using a
	// different user is rejected as forbidden.
	// +optional
	UserID string `json:"userID,omitempty"`
	// UserPrincipal holds the information about the ext auth provider
	// managed user (principal) owning the token.
	UserPrincipal TokenPrincipal `json:"userPrincipal"`
	// Kind describes the nature of the token. The value "session" indicates
	// a login/session token. Any other value, including the empty string,
	// which is the default, stands for some kind of derived token.
	// +optional
	Kind string `json:"kind"`
	// Human readable free-form description of the token. For example, its purpose.
	// +optional
	Description string `json:"description,omitempty"`
	// TTL is the time-to-live of the token, in milliseconds.
	// Setting a value < 0 represents +infinity, i.e. a token which does not expire.
	// The default is indicated by the value `0`.
	// This default is provided by the `auth-token-max-ttl-minutes` setting.
	// Note that this default is also the maximum specifiable TTL.
	// +optional
	TTL int64 `json:"ttl"`
	// Enabled indicates an active token. The default (`null`) indicates an
	// enabled token.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// TokenPrincipal contains the data about the user principal owning the token.
type TokenPrincipal struct {
	// Name is the name unique to the authentication provider.
	Name string `json:"name"`
	// DisplayName is the human readable name/description of the principal.
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// LoginName is the account name of the principal in the managing auth provider.
	LoginName string `json:"loginName,omitempty"`
	// ProfilePicture is an url to a picture to use when displaying the principal.
	// +optional
	ProfilePicture string `json:"profilePicture,omitempty"`
	// ProfileURL is not used by the system
	// +optional
	ProfileURL string `json:"profileURL,omitempty"`
	// PrincipalType is the kind of principal. Legal values are "user" and "group".
	PrincipalType string `json:"principalType,omitempty"`
	// Me is a virtual flag for use with the dashboard.
	Me bool `json:"me,omitempty"`
	// MemberOf is a virtual flag for use with the dashboard.
	MemberOf bool `json:"memberOf,omitempty"`
	// Provider is the name of the auth provider managing the principal
	Provider string `json:"provider,omitempty"`
	// ExtraInfo contains additional information about the principal.
	ExtraInfo map[string]string `json:"extraInfo,omitempty"`
}

// TokenStatus defines the most recently observed status of the Token.
type TokenStatus struct {
	// Value is the access key. It is shown only on token creation and not saved.
	Value string `json:"tokenValue,omitempty"`
	// Hash is the hash of the Value.
	Hash string `json:"tokenHash,omitempty"`
	// Current indicates whether the token was used to authenticate the current request.
	Current bool `json:"current"`
	// Expired indicates whether the token has exceeded its TTL.
	Expired bool `json:"expired"`
	// ExpiresAt is the token's expiration timestamp or an empty string if the token doesn't expire.
	ExpiresAt string `json:"expiresAt"`
	// LastUpdateTime is the timestamp of the last change to the token.
	LastUpdateTime string `json:"lastUpdateTime"`
	// LastUsedAt is the timestamp of the last time the token was used to authenticate.
	LastUsedAt *metav1.Time `json:"lastUsedAt,omitempty"`
	// LastActivitySeen is the timestamp of the last time user activity
	// (mouse movement, interaction, ...) was reported for the token.
	LastActivitySeen *metav1.Time `json:"lastActivitySeen,omitempty"`
}

// Implement the TokenAccessor interface

func (t *Token) GetName() string {
	return t.Name
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
	return t.Spec.UserPrincipal.Provider
}
func (t *Token) GetUserPrincipal() apiv3.Principal {
	return apiv3.Principal{
		TypeMeta:       metav1.TypeMeta{},
		ObjectMeta:     metav1.ObjectMeta{Name: t.Spec.UserPrincipal.Name},
		DisplayName:    t.Spec.UserPrincipal.DisplayName,
		LoginName:      t.Spec.UserPrincipal.LoginName,
		ProfilePicture: t.Spec.UserPrincipal.ProfilePicture,
		ProfileURL:     t.Spec.UserPrincipal.ProfileURL,
		PrincipalType:  t.Spec.UserPrincipal.PrincipalType,
		Me:             t.Spec.UserPrincipal.Me,
		MemberOf:       t.Spec.UserPrincipal.MemberOf,
		Provider:       t.Spec.UserPrincipal.Provider,
		ExtraInfo:      t.Spec.UserPrincipal.ExtraInfo,
	}
}

func (t *Token) GetGroupPrincipals() []apiv3.Principal {
	// Not supported by ext tokens.
	return []apiv3.Principal{}
}

func (t *Token) GetProviderInfo() map[string]string {
	// Not supported by ext tokens.
	return map[string]string{}
}

func (t *Token) GetLastUsedAt() *metav1.Time {
	return t.Status.LastUsedAt
}

func (t *Token) GetLastActivitySeen() *metav1.Time {
	return t.Status.LastActivitySeen
}

func (t *Token) GetCreationTime() metav1.Time {
	return t.CreationTimestamp
}
