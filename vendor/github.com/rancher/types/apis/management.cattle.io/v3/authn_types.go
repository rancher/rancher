package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Token struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	TokenID                  string `json:"tokenId,omitempty"`
	TokenValue               string `json:"tokenValue,omitempty"`
	User                     string `json:"user,omitempty"`
	ExternalID               string `json:"externalId,omitempty"`
	AuthProvider             string `json:"authProvider,omitempty"`
	TTLMillis                string `json:"ttl,omitempty"`
	IdentityRefreshTTLMillis string `json:"identityRefreshTTL,omitempty"`
	LastUpdateTime           string `json:"lastUpdateTime,omitempty"`
	IsDerived                bool   `json:"isDerived,omitempty"`
	Description              string `json:"description,omitempty"`
}

type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Secret     string `json:"secret,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
}

type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type GroupMember struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	GroupName  string `json:"groupName,omitempty" norman:"type=reference[/v3/schemas/group]"`
	ExternalID string `json:"externalId,omitempty"`
}

type Identity struct {
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
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	TTLMillis                string           `json:"ttl,omitempty"`
	IdentityRefreshTTLMillis string           `json:"identityRefreshTTL,omitempty"`
	Description              string           `json:"description,omitempty"`
	ResponseType             string           `json:"responseType,omitempty"` //json or cookie
	LocalCredential          LocalCredential  `json:"localCredential, omitempty"`
	GithubCredential         GithubCredential `json:"githubCredential, omitempty"`
}

//LocalCredential stores the local auth creds
type LocalCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Username string `json:"username"`
	Password string `json:"password"`
}

//GithubCredential stores the github auth creds
type GithubCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Code string `json:"code"`
}
