package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Token           string            `json:"token" norman:"writeOnly,noupdate"`
	UserPrincipal   Principal         `json:"userPrincipal" norman:"type=reference[principal]"`
	GroupPrincipals []Principal       `json:"groupPrincipals" norman:"type=array[reference[principal]]"`
	ProviderInfo    map[string]string `json:"providerInfo,omitempty"`
	UserID          string            `json:"userId" norman:"type=reference[user]"`
	AuthProvider    string            `json:"authProvider"`
	TTLMillis       int64             `json:"ttl"`
	LastUpdateTime  string            `json:"lastUpdateTime"`
	IsDerived       bool              `json:"isDerived"`
	Description     string            `json:"description"`
	Expired         bool              `json:"expired"`
	ExpiresAt       string            `json:"expiresAt"`
}

type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName        string   `json:"displayName,omitempty"`
	Description        string   `json:"description"`
	Username           string   `json:"username,omitempty"`
	Password           string   `json:"password,omitempty" norman:"writeOnly,noupdate"`
	MustChangePassword bool     `json:"mustChangePassword,omitempty"`
	PrincipalIDs       []string `json:"principalIds,omitempty" norman:"type=array[reference[principal]]"`
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
	PrincipalID string `json:"principalId,omitempty" norman:"type=reference[principal]"`
}

type Principal struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	DisplayName    string            `json:"displayName,omitempty"`
	LoginName      string            `json:"loginName,omitempty"`
	ProfilePicture string            `json:"profilePicture,omitempty"`
	ProfileURL     string            `json:"profileURL,omitempty"`
	PrincipalType  string            `json:"principalType,omitempty"`
	Me             bool              `json:"me,omitempty"`
	MemberOf       bool              `json:"memberOf,omitempty"`
	Provider       string            `json:"provider,omitempty"`
	ExtraInfo      map[string]string `json:"extraInfo,omitempty"`
}

type SearchPrincipalsInput struct {
	Name          string `json:"name" norman:"type=string,required,notnullable"`
	PrincipalType string `json:"principalType,omitempty" norman:"type=enum,options=user|group"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword" norman:"type=string,required"`
	NewPassword     string `json:"newPassword" norman:"type=string,required"`
}

type SetPasswordInput struct {
	NewPassword string `json:"newPassword" norman:"type=string,required"`
}

type AuthConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Type                string   `json:"type" norman:"noupdate"`
	Enabled             bool     `json:"enabled,omitempty"`
	AccessMode          string   `json:"accessMode,omitempty" norman:"required,notnullable,type=enum,options=required|restricted|unrestricted"`
	AllowedPrincipalIDs []string `json:"allowedPrincipalIds,omitempty" norman:"type=array[reference[principal]]"`
}

type LocalConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline" mapstructure:",squash"`
}

type GithubConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline" mapstructure:",squash"`

	Hostname     string `json:"hostname,omitempty" norman:"default=github.com" norman:"noupdate"`
	TLS          bool   `json:"tls,omitempty" norman:"notnullable,default=true" norman:"noupdate"`
	ClientID     string `json:"clientId,omitempty" norman:"noupdate"`
	ClientSecret string `json:"clientSecret,omitempty" norman:"noupdate,type=password"`
}

type GithubConfigTestOutput struct {
	RedirectURL string `json:"redirectUrl"`
}

type GithubConfigApplyInput struct {
	GithubConfig GithubConfig `json:"githubConfig, omitempty"`
	Code         string       `json:"code,omitempty"`
	Enabled      bool         `json:"enabled,omitempty"`
}

type ActiveDirectoryConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline" mapstructure:",squash"`

	Servers                     []string `json:"servers,omitempty"                     norman:"type=array[string],required"`
	Port                        int64    `json:"port,omitempty"                        norman:"default=389"`
	TLS                         bool     `json:"tls,omitempty"                         norman:"default=false"`
	Certificate                 string   `json:"certificate,omitempty"`
	DefaultLoginDomain          string   `json:"defaultLoginDomain,omitempty"`
	ServiceAccountUsername      string   `json:"serviceAccountUsername,omitempty"      norman:"required"`
	ServiceAccountPassword      string   `json:"serviceAccountPassword,omitempty"      norman:"type=password,required"`
	UserDisabledBitMask         int64    `json:"userDisabledBitMask,omitempty"         norman:"default=2"`
	UserSearchBase              string   `json:"userSearchBase,omitempty"              norman:"required"`
	UserSearchAttribute         string   `json:"userSearchAttribute,omitempty"         norman:"default=sAMAccountName|sn|givenName,required"`
	UserLoginAttribute          string   `json:"userLoginAttribute,omitempty"          norman:"default=sAMAccountName,required"`
	UserObjectClass             string   `json:"userObjectClass,omitempty"             norman:"default=person,required"`
	UserNameAttribute           string   `json:"userNameAttribute,omitempty"           norman:"default=name,required"`
	UserEnabledAttribute        string   `json:"userEnabledAttribute,omitempty"        norman:"default=userAccountControl,required"`
	GroupSearchBase             string   `json:"groupSearchBase,omitempty"`
	GroupSearchAttribute        string   `json:"groupSearchAttribute,omitempty"        norman:"default=sAMAccountName,required"`
	GroupObjectClass            string   `json:"groupObjectClass,omitempty"            norman:"default=group,required"`
	GroupNameAttribute          string   `json:"groupNameAttribute,omitempty"          norman:"default=name,required"`
	GroupDNAttribute            string   `json:"groupDNAttribute,omitempty"            norman:"default=distinguishedName,required"`
	GroupMemberUserAttribute    string   `json:"groupMemberUserAttribute,omitempty"    norman:"default=distinguishedName,required"`
	GroupMemberMappingAttribute string   `json:"groupMemberMappingAttribute,omitempty"`
	ConnectionTimeout           int64    `json:"connectionTimeout,omitempty"           norman:"default=5000"`
}

type ActiveDirectoryTestAndApplyInput struct {
	ActiveDirectoryConfig ActiveDirectoryConfig `json:"activeDirectoryConfig, omitempty"`
	Username              string                `json:"username"`
	Password              string                `json:"password"`
	Enabled               bool                  `json:"enabled,omitempty"`
}

type AzureADConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline" mapstructure:",squash"`

	TenantID     string `json:"tenantId,omitempty"                          norman:"required"`
	ClientID     string `json:"clientId,omitempty"                          norman:"required"`
	Domain       string `json:"domain,omitempty"                            norman:"required"`
	ClientSecret string `json:"clientSecret,omitempty"`
}

type AzureADTestAndApplyInput struct {
	AzureADConfig AzureADConfig `json:"azureAdConfig, omitempty"`
	Username      string        `json:"username"`
	Password      string        `json:"password"`
	Enabled       bool          `json:"enabled,omitempty"`
}
