package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuthProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type string `json:"type"`
}

func (a *AuthProvider) GetType() string {
	return a.Type
}

// OAuthProvider contains the OAuth configuration of the AuthProvider
type OAuthProvider struct {
	ClientID      string   `json:"clientId"`
	Scopes        []string `json:"scopes"`
	OAuthEndpoint `json:",inline"`
}

// OAuthEndpoint contains the endpoints needed for an oauth exchange.
// See also https://pkg.go.dev/golang.org/x/oauth2#Endpoint
type OAuthEndpoint struct {
	AuthURL       string `json:"authUrl,omitempty"`
	DeviceAuthURL string `json:"deviceAuthUrl,omitempty"`
	TokenURL      string `json:"tokenUrl,omitempty"`
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

	RedirectURL string `json:"redirectUrl"`
}

type GithubLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
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
	Code         string `json:"code" norman:"type=string,required"`
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
	TenantID    string `json:"tenantId,omitempty"`

	OAuthProvider `json:",inline"`
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
	IDToken      string `json:"id_token,omitempty"`
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

// +genclient
// +kubebuilder:skipversion
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type GenericOIDCProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthProvider      `json:",inline"`

	RedirectURL string `json:"redirectUrl"`
	Scopes      string `json:"scopes"`
}

type GenericOIDCLogin struct {
	GenericLogin `json:",inline"`
	Code         string `json:"code" norman:"type=string,required"`
}
