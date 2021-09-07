package client

const (
	ActiveDirectoryTestAndApplyInputType                       = "activeDirectoryTestAndApplyInput"
	ActiveDirectoryTestAndApplyInputFieldActiveDirectoryConfig = "activeDirectoryConfig"
	ActiveDirectoryTestAndApplyInputFieldEnabled               = "enabled"
	ActiveDirectoryTestAndApplyInputFieldPassword              = "password"
	ActiveDirectoryTestAndApplyInputFieldUsername              = "username"
)

type ActiveDirectoryTestAndApplyInput struct {
	ActiveDirectoryConfig *ActiveDirectoryConfig `json:"activeDirectoryConfig,omitempty" yaml:"activeDirectoryConfig,omitempty"`
	Enabled               bool                   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Password              string                 `json:"password,omitempty" yaml:"password,omitempty"`
	Username              string                 `json:"username,omitempty" yaml:"username,omitempty"`
}
