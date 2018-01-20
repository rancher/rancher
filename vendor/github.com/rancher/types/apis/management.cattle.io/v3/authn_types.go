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
	TTLMillis        int              `json:"ttl,omitempty"`
	Description      string           `json:"description,omitempty"`
	ResponseType     string           `json:"responseType,omitempty"` //json or cookie
	LocalCredential  LocalCredential  `json:"localCredential, omitempty"`
	GithubCredential GithubCredential `json:"githubCredential, omitempty"`
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
	NewPassword string `json:"newPassword"`
}
