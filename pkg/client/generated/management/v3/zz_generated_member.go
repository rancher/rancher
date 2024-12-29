package client

const (
	MemberType                  = "member"
	MemberFieldAccessType       = "accessType"
	MemberFieldDisplayName      = "displayName"
	MemberFieldGroupPrincipalID = "groupPrincipalId"
	MemberFieldUserID           = "userId"
	MemberFieldUserPrincipalID  = "userPrincipalId"
)

type Member struct {
	AccessType       string `json:"accessType,omitempty" yaml:"accessType,omitempty"`
	DisplayName      string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	GroupPrincipalID string `json:"groupPrincipalId,omitempty" yaml:"groupPrincipalId,omitempty"`
	UserID           string `json:"userId,omitempty" yaml:"userId,omitempty"`
	UserPrincipalID  string `json:"userPrincipalId,omitempty" yaml:"userPrincipalId,omitempty"`
}
