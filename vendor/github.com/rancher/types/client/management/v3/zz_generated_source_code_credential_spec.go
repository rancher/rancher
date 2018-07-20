package client

const (
	SourceCodeCredentialSpecType                = "sourceCodeCredentialSpec"
	SourceCodeCredentialSpecFieldAccessToken    = "accessToken"
	SourceCodeCredentialSpecFieldAvatarURL      = "avatarUrl"
	SourceCodeCredentialSpecFieldClusterID      = "clusterId"
	SourceCodeCredentialSpecFieldDisplayName    = "displayName"
	SourceCodeCredentialSpecFieldHTMLURL        = "htmlUrl"
	SourceCodeCredentialSpecFieldLoginName      = "loginName"
	SourceCodeCredentialSpecFieldSourceCodeType = "sourceCodeType"
	SourceCodeCredentialSpecFieldUserID         = "userId"
)

type SourceCodeCredentialSpec struct {
	AccessToken    string `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
	AvatarURL      string `json:"avatarUrl,omitempty" yaml:"avatarUrl,omitempty"`
	ClusterID      string `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	DisplayName    string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	HTMLURL        string `json:"htmlUrl,omitempty" yaml:"htmlUrl,omitempty"`
	LoginName      string `json:"loginName,omitempty" yaml:"loginName,omitempty"`
	SourceCodeType string `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	UserID         string `json:"userId,omitempty" yaml:"userId,omitempty"`
}
