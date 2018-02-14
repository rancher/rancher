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
	ClusterId              string    `json:"clusterId,omitempty"`
	Language               string    `json:"language,omitempty"`
	Permissions            *RepoPerm `json:"permissions,omitempty"`
	SourceCodeCredentialId string    `json:"sourceCodeCredentialId,omitempty"`
	SourceCodeType         string    `json:"sourceCodeType,omitempty"`
	URL                    string    `json:"url,omitempty"`
	UserId                 string    `json:"userId,omitempty"`
}
