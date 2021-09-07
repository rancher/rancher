package client

const (
	SourceCodeRepositorySpecType                        = "sourceCodeRepositorySpec"
	SourceCodeRepositorySpecFieldDefaultBranch          = "defaultBranch"
	SourceCodeRepositorySpecFieldLanguage               = "language"
	SourceCodeRepositorySpecFieldPermissions            = "permissions"
	SourceCodeRepositorySpecFieldProjectID              = "projectId"
	SourceCodeRepositorySpecFieldSourceCodeCredentialID = "sourceCodeCredentialId"
	SourceCodeRepositorySpecFieldSourceCodeType         = "sourceCodeType"
	SourceCodeRepositorySpecFieldURL                    = "url"
	SourceCodeRepositorySpecFieldUserID                 = "userId"
)

type SourceCodeRepositorySpec struct {
	DefaultBranch          string    `json:"defaultBranch,omitempty" yaml:"defaultBranch,omitempty"`
	Language               string    `json:"language,omitempty" yaml:"language,omitempty"`
	Permissions            *RepoPerm `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	ProjectID              string    `json:"projectId,omitempty" yaml:"projectId,omitempty"`
	SourceCodeCredentialID string    `json:"sourceCodeCredentialId,omitempty" yaml:"sourceCodeCredentialId,omitempty"`
	SourceCodeType         string    `json:"sourceCodeType,omitempty" yaml:"sourceCodeType,omitempty"`
	URL                    string    `json:"url,omitempty" yaml:"url,omitempty"`
	UserID                 string    `json:"userId,omitempty" yaml:"userId,omitempty"`
}
