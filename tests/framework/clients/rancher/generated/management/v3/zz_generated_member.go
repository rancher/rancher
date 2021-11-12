package client

const (
	MemberType                  = "member"
	MemberFieldAccessType       = "accessType"
	MemberFieldGroupPrincipalID = "groupPrincipalId"
	MemberFieldUserPrincipalID  = "userPrincipalId"
)

type Member struct {
	AccessType       string `json:"accessType,omitempty" yaml:"accessType,omitempty"`
	GroupPrincipalID string `json:"groupPrincipalId,omitempty" yaml:"groupPrincipalId,omitempty"`
	UserPrincipalID  string `json:"userPrincipalId,omitempty" yaml:"userPrincipalId,omitempty"`
}
