package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OAuthEndpoint struct {
	AuthURL       string `json:"authUrl"`
	DeviceAuthURL string `json:"deviceAuthUrl"`
	TokenURL      string `json:"tokenUrl"`
}

type OAuthAuthorizationInfo struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RedirectURL  string `json:"redirectUrl"`
}

type OAuthDeviceInfo struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuthProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type string `json:"type"`

	Scopes    []string      `json:"scopes"`
	Endpoints OAuthEndpoint `json:"endpoints"`

	// AuthClientInfo is the info required for the Authorization Code grant. It
	// is optional because not every provider supports it.
	AuthClientInfo *OAuthAuthorizationInfo `json:"authClientInfo,omitempty"`

	// DeviceClientInfo is the info required for the Device Code grant.
	// It is optional because not every provider supports it.
	DeviceClientInfo *OAuthDeviceInfo `json:"deviceClientInfo,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuthToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Token     string `json:"token"`
	ExpiresAt string `json:"expiresAt"`
}

type GenericLogin struct {
	TTLMillis    int64  `json:"ttl,omitempty"`
	Description  string `json:"description,omitempty" norman:"type=string,required"`
	ResponseType string `json:"responseType,omitempty" norman:"type=string,required"` //json or cookie
}

type BasicLogin struct {
	GenericLogin `json:",inline"`
	Username     string `json:"username" norman:"type=string,required"`
	Password     string `json:"password" norman:"type=string,required"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type LocalProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GithubProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	// RedirectURL is for the UI flow which will be different than the CLI one
	RedirectURL string `json:"redirectUrl"`
}

type GithubLogin struct {
	GenericLogin `json:",inline"`

	// AccessToken is the Oauth 2.0 access token received after an
	// authentication flow.
	AccessToken string `json:"access_token" norman:"type=string"`

	// Deprecated: Send us an access token instead.
	Code string `json:"code" norman:"type=string"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GoogleOAuthProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

type GoogleOauthLogin struct {
	GenericLogin `json:",inline"`

	// AccessToken is the Oauth 2.0 access token received after an
	// authentication flow.
	AccessToken string `json:"access_token" norman:"type=string"`

	// Deprecated: Send us an access token instead.
	Code string `json:"code" norman:"type=string,required"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ActiveDirectoryProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	DefaultLoginDomain string `json:"defaultLoginDomain,omitempty"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AzureADProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SamlProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

type AzureADLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OpenLdapProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type FreeIpaProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`
}

type PingProvider struct {
	SamlProvider `json:",inline"`
}

type ShibbolethProvider struct {
	SamlProvider `json:",inline"`
}

type ADFSProvider struct {
	SamlProvider `json:",inline"`
}

type KeyCloakProvider struct {
	SamlProvider `json:",inline"`
}

type OKTAProvider struct {
	SamlProvider `json:",inline"`
}

type SamlLoginInput struct {
	FinalRedirectURL string `json:"finalRedirectUrl"`
	RequestID        string `json:"requestId"`
	PublicKey        string `json:"publicKey"`
	ResponseType     string `json:"responseType"`
}

type SamlLoginOutput struct {
	IdpRedirectURL string `json:"idpRedirectUrl"`
}

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OIDCProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
}

type OIDCLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
}

type KeyCloakOIDCProvider struct {
	OIDCProvider `json:",inline"`
}
