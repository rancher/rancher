package client

const (
	SourceCodeRepositorySpecType                        = "sourceCodeRepositorySpec"
	SourceCodeRepositorySpecFieldClusterId              = "clusterId"
	SourceCodeRepositorySpecFieldLanguage               = "language"
	SourceCodeRepositorySpecFieldPermissions            = "permissions"
	SourceCodeRepositorySpecFieldSourceCodeCredentialId = "sourceCodeCredentialId"
	SourceCodeRepositorySpecFieldSourceCodeType         = "sourceCodeType"
	SourceCodeRepositorySpecFieldURL                    = "url"
	SourceCodeRepositorySpecFieldUserId                 = "userId"
)

type SourceCodeRepositorySpec struct {
	ClusterId              string    `json:"clusterId,omitempty" yaml:"clusterId,omitempty"`
	Language               string    `json:"language,omitempty" yaml:"language,omitempty"`
	Permissions            *RepoPerm `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	SourceCodeCredentialId string    `json:"sourceCodeCredentialId,omitempty" yaml:"sourceCodeCredentialId,omitempty"`
	SourceCodeType         string    `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	URL                    string    `json:"url,omitempty" yaml:"url,omitempty"`
	UserId                 string    `json:"userId,omitempty" yaml:"userId,omitempty"`
}
