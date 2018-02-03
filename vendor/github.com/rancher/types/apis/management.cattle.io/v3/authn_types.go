package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Token           string            `json:"token" norman:"writeOnly,noupdate"`
	UserPrincipal   Principal         `json:"userPrincipal" norman:"type=reference[Principal]"`
	GroupPrincipals []Principal       `json:"groupPrincipals" norman:"type=array[reference[Principal]]"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty"`
	UserID          string            `json:"userId" norman:"type=reference[User]"`
	AuthProvider    string            `json:"authProvider"`
	TTLMillis       int               `json:"ttl"`
	LastUpdateTime  string            `json:"lastUpdateTime"`
	IsDerived       bool              `json:"isDerived"`
	Description     string            `json:"description"`
}

type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName        string   `json:"displayName,omitempty"`
	Description        string   `json:"description"`
	Username           string   `json:"username,omitempty"`
	Password           string   `json:"password,omitempty" norman:"writeOnly,noupdate"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	PrincipalIDs       []string `json:"principalIds,omitempty" norman:"type=array[reference[Principal]]"`
	Me                 bool     `json:"me,omitempty"`
}

type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName string `json:"displayName,omitempty"`
}

type GroupMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	GroupName   string `json:"groupName,omitempty" norman:"type=reference[group]"`
	PrincipalID string `json:"principalId,omitempty" norman:"type=reference[Principal]"`
}

type Principal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName    string            `json:"displayName,omitempty"`
	LoginName      string            `json:"loginName,omitempty"`
	ProfilePicture string            `json:"profilePicture,omitempty"`
	ProfileURL     string            `json:"profileURL,omitempty"`
	Kind           string            `json:"kind,omitempty"`
	Me             bool              `json:"me,omitempty"`
	MemberOf       bool              `json:"memberOf,omitempty"`
	ExtraInfo      map[string]string `json:"extraInfo,omitempty"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword" norman:"type=string,required"`
	NewPassword     string `json:"newPassword" norman:"type=string,required"`
}

type SetPasswordInput struct {
	NewPassword string `json:"newPassword" norman:"type=string,required"`
}

//AuthConfig structure contains the AuthConfig definition
type AuthConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type string `json:"type"`
}

//GithubConfig structure contains the github config definition
type GithubConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline"`

	Hostname     string `json:"hostname,omitempty"`
	Scheme       string `json:"scheme,omitempty"`
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	Enabled      bool   `json:"enabled,omitempty"`
}

//LocalConfig structure contains the local config definition
type LocalConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

//GithubConfigTestInput structure defines all properties that can be sent by client to configure github
type GithubConfigTestInput struct {
	GithubConfig GithubConfig `json:"githubConfig, omitempty"`
	Enabled      bool         `json:"enabled,omitempty"`
}

//GithubConfigApplyInput structure defines all properties that can be sent by client to configure github
type GithubConfigApplyInput struct {
	GithubConfig GithubConfig `json:"githubConfig, omitempty"`
	Code         string       `json:"code,omitempty"`
	Enabled      bool         `json:"enabled,omitempty"`
}
