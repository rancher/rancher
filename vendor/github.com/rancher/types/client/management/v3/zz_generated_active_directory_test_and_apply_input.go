package client

const (
	ActiveDirectoryTestAndApplyInputType                       = "activeDirectoryTestAndApplyInput"
	ActiveDirectoryTestAndApplyInputFieldActiveDirectoryConfig = "activeDirectoryConfig"
	ActiveDirectoryTestAndApplyInputFieldEnabled               = "enabled"
	ActiveDirectoryTestAndApplyInputFieldPassword              = "password"
	ActiveDirectoryTestAndApplyInputFieldUsername              = "username"
)

type ActiveDirectoryTestAndApplyInput struct {
	ActiveDirectoryConfig *ActiveDirectoryConfig `json:"activeDirectoryConfig,omitempty"`
	Enabled               bool                   `json:"enabled,omitempty"`
	Password              string                 `json:"password,omitempty"`
	Username              string                 `json:"username,omitempty"`
}
