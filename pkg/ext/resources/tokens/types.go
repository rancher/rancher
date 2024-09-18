package tokens

import (
	apiv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Token is the new extension Token structure
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
	// Human readable description
	// +optional
	Description string `json:"description, omitempty"`
	// ClusterName is the cluster that the token is scoped to. If empty, the token
	// can be used for all clusters.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// TTL is the time-to-live of the token, in milliseconds
	TTL int64 `json:"ttl"`
	// Enabled indicates an active token
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// Indicates a token which was derived from some other token
	IsDerived bool `json:"isDerived"`
}

// TokenStatus contains the data derived from the specification or otherwise generated.
type TokenStatus struct {
	// TokenValue is the access key. Shown only on token creation. Not saved.
	TokenValue string `json:"tokenValue,omitempty"`
	// TokenHash is the hash of the value. Only thing saved.
	TokenHash string `json:"tokenHash,omitempty"`

	// Time derived data

	// The time-derived fields are not stored in the backing secret. Both
	// values can be trivially computed from the secret's/token's creation
	// time, the time to live, and the current time.

	// Expired flag, derived from creation time and time-to-live
	Expired bool `json:"expired"`
	// ExpiredAt is creation time + time-to-live
	ExpiredAt string `json:"expiredAt"`

	// User derived data

	// The user derived information is complex to determine. As such this is
	// stored in the backing secret to avoid recomputing it whenever the
	// token is retrieved.

	// _________ XXX RESEARCH NOTES XXX _________________________ START
	//
	// For the norman tokens the User- and GroupPrincipals (UP, GPs) are
	// provided by the calling context, obviating the need to compute it as
	// part of the token creation. The auth provider is the UP's provider,
	// i.e. trivially found.
	//
	// See `NewLoginToken` (token/manager.go), and `HandleSamlAssertion`
	// (saml_client.go) as an example calling context. In that context UP
	// and GPs are created from the data in the received SAML assertion.
	// While the UP is stored in the User (*) the GPs are given to the new
	// token and also stored in a UserAttribute resource.
	//
	// The other external auth providers operate in a similar manner. See
	// `AuthenticateUser` for Principal creation, and `createLoginToken` for
	// the calling context passing this data to `NewLoginToken`.
	//
	// The providers differ in what fields of the Principal structures they
	// fill in. I.e. the data is provider-dependent.
	//
	// (*) Not truly true, only the principalID is stored, nothing else.
	//
	// ATTENTION --- Which means that the full UserPrincipal information is
	// only available in the norman token itself.
	//
	// NOTE that the derived tokens get their UP from the base token they
	// are derived from. See `createDerivedToken` (tokens/manager.go).
	//
	// =========== THIS IS A PROBLEM FOR THE EXT TOKENS =================
	//
	// While the GPs are retrievable from the `UserAttribute` resource
	// associated with the User the new token will be for, getting the UP is
	// difficult.
	//
	// While we should be able to get and use the UP data for the user
	// making the Create request (via its token) that is sensible only if
	// the user X making the request is the same user X the new token will
	// be for. The moment we have an admin User X requesting the creation of
	// a token for some other user Y we have no source for UP data.
	//
	// Well, maybe. If we want to reach into hackish territory we could try
	// and see if user Y already has tokens around for it, of either kind,
	// and if yes, pull the UP from any of these,
	//
	// BEWARE that this further depends on the custom handler having proper
	// access to the token which auth'd the request. The current imp. API
	// foundation does not seem to provide this. At the moment we can and do
	// retrieve the userID from the provided `user.Info` structure.
	//
	// SOLUTION ?? Extend the existing norman types and auth/token code to
	// store the UP in the UserAttribute as well, making it trivially
	// available.
	//
	// [[ Digression: A token created for user X by user X can be considered
	//    derived. Everything else would be non-derived => Set the flag ]]
	//
	// ANOTHER PROBLEM is the `ProviderInfo`. So far I have not managed to
	// determine where this information is coming from at all. While it is
	// copied around I found no originating places yet.
	//
	// _________ XXX RESEARCH NOTES XXX _________________________ END

	// AuthProvider names the auth provider managing the user
	AuthProvider string `json:"authProvider"`
	// UserPrincipal holds the detailed user data
	UserPrincipal apiv3.Principal `json:"userPrincipal"`

	// GroupPrincipals holds detailed group information
	// This is not supported here.
	// The primary location for this information are the UserAttribute resources.
	// The norman Token maintains GPs only as legacy.
	// The ext tokens here shed this legacy.

	// ProviderInfo provides provider-specific details
	ProviderInfo map[string]string `json:"providerInfo"`

	// Time of last change to the token
	LastUpdateTime string `json:"lastUpdateTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TokenList is the standard structure for a list of Tokens in the kube API
type TokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Token `json:"items"`
}

// Implement the TokenAccessor interface

func (t *Token) GetName() string {
	return t.ObjectMeta.Name
}

func (t *Token) IsEnabled() bool {
	return t.Spec.Enabled
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
	return t.Status.UserPrincipal
}

func (t *Token) GetGroupPrincipals() []apiv3.Principal {
	return nil
}

func (t *Token) GetProviderInfo() map[string]string {
	return t.Status.ProviderInfo
}
