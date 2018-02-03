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

//LoginInput structure defines all properties that can be sent by client to create a token
type LoginInput struct {
	TTLMillis                 int                       `json:"ttl,omitempty"`
	Description               string                    `json:"description,omitempty"`
	ResponseType              string                    `json:"responseType,omitempty"` //json or cookie
	LocalCredential           LocalCredential           `json:"localCredential, omitempty"`
	GithubCredential          GithubCredential          `json:"githubCredential, omitempty"`
	ActiveDirectoryCredential ActiveDirectoryCredential `json:"activeDirectoryCredential, omitempty"`
}

//LocalCredential stores the local auth creds
type LocalCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

//GithubCredential stores the github auth creds
type GithubCredential struct {
	Code string `json:"code"`
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
	ClientSecret string `json:"clientSecret,omitempty" norman:"writeOnly"`
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
	GithubConfig     GithubConfig     `json:"githubConfig, omitempty"`
	GithubCredential GithubCredential `json:"githubCredential, omitempty"`
	Enabled          bool             `json:"enabled,omitempty"`
}

//ActiveDirectoryCredential stores the ActiveDirectory auth creds
type ActiveDirectoryCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

//ActiveDirectoryConfig structure contains the ActiveDirectory config definition
type ActiveDirectoryConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	AuthConfig        `json:",inline"`

	Server                      string `json:"server,omitempty"`
	Port                        int64  `json:"port,omitempty"`
	UserDisabledBitMask         int64  `json:"userDisabledBitMask,omitempty"`
	LoginDomain                 string `json:"loginDomain,omitempty"`
	Domain                      string `json:"domain,omitempty"`
	GroupSearchDomain           string `json:"groupSearchDomain,omitempty"`
	ServiceAccountUsername      string `json:"serviceAccountUsername,omitempty"`
	ServiceAccountPassword      string `json:"serviceAccountPassword,omitempty" norman:"writeOnly"`
	TLS                         bool   `json:"tls,omitempty"`
	UserSearchField             string `json:"userSearchField,omitempty"`
	UserLoginField              string `json:"userLoginField,omitempty"`
	UserObjectClass             string `json:"userObjectClass,omitempty"`
	UserNameField               string `json:"userNameField,omitempty"`
	UserEnabledAttribute        string `json:"userEnabledAttribute,omitempty"`
	GroupSearchField            string `json:"groupSearchField,omitempty"`
	GroupObjectClass            string `json:"groupObjectClass,omitempty"`
	GroupNameField              string `json:"groupNameField,omitempty"`
	GroupDNField                string `json:"groupDNField,omitempty"`
	GroupMemberUserAttribute    string `json:"groupMemberUserAttribute,omitempty"`
	GroupMemberMappingAttribute string `json:"groupMemberMappingAttribute,omitempty"`
	ConnectionTimeout           int64  `json:"connectionTimeout,omitempty"`
	Enabled                     bool   `json:"enabled,omitempty"`
}

//ActiveDirectoryConfigApplyInput structure defines all properties that can be sent by client to configure activedirectory
type ActiveDirectoryConfigApplyInput struct {
	ActiveDirectoryConfig     ActiveDirectoryConfig     `json:"activeDirectoryConfig, omitempty"`
	ActiveDirectoryCredential ActiveDirectoryCredential `json:"activeDirectoryCredential, omitempty"`
	Enabled                   bool                      `json:"enabled,omitempty"`
}
