package client

const (
	SourceCodeCredentialSpecType                = "sourceCodeCredentialSpec"
	SourceCodeCredentialSpecFieldAccessToken    = "accessToken"
	SourceCodeCredentialSpecFieldAvatarURL      = "avatarUrl"
	SourceCodeCredentialSpecFieldDisplayName    = "displayName"
	SourceCodeCredentialSpecFieldGitLoginName   = "gitLoginName"
	SourceCodeCredentialSpecFieldHTMLURL        = "htmlUrl"
	SourceCodeCredentialSpecFieldLoginName      = "loginName"
	SourceCodeCredentialSpecFieldProjectID      = "projectId"
	SourceCodeCredentialSpecFieldSourceCodeType = "sourceCodeType"
	SourceCodeCredentialSpecFieldUserID         = "userId"
)

type SourceCodeCredentialSpec struct {
	AccessToken    string `json:"accessToken,omitempty" yaml:"accessToken,omitempty"`
	AvatarURL      string `json:"avatarUrl,omitempty" yaml:"avatarUrl,omitempty"`
	DisplayName    string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	GitLoginName   string `json:"gitLoginName,omitempty" yaml:"gitLoginName,omitempty"`
	HTMLURL        string `json:"htmlUrl,omitempty" yaml:"htmlUrl,omitempty"`
	LoginName      string `json:"loginName,omitempty" yaml:"loginName,omitempty"`
	ProjectID      string `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SourceCodeType string `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	UserID         string `json:"userId,omitempty" yaml:"userId,omitempty"`
}
