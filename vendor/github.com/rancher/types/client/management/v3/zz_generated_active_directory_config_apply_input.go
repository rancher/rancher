package client

const (
	ActiveDirectoryConfigApplyInputType                           = "activeDirectoryConfigApplyInput"
	ActiveDirectoryConfigApplyInputFieldActiveDirectoryConfig     = "activeDirectoryConfig"
	ActiveDirectoryConfigApplyInputFieldActiveDirectoryCredential = "activeDirectoryCredential"
	ActiveDirectoryConfigApplyInputFieldEnabled                   = "enabled"
)

type ActiveDirectoryConfigApplyInput struct {
	ActiveDirectoryConfig     *ActiveDirectoryConfig     `json:"activeDirectoryConfig,omitempty"`
	ActiveDirectoryCredential *ActiveDirectoryCredential `json:"activeDirectoryCredential,omitempty"`
	Enabled                   *bool                      `json:"enabled,omitempty"`
}
