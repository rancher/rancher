package client

const (
	SambaBoxTestAndApplyInputType                = "sambaBoxTestAndApplyInput"
	SambaBoxTestAndApplyInputFieldSambaBoxConfig = "sambaBoxConfig"
	SambaBoxTestAndApplyInputFieldEnabled        = "enabled"
	SambaBoxTestAndApplyInputFieldPassword       = "password"
	SambaBoxTestAndApplyInputFieldUsername       = "username"
)

type SambaBoxTestAndApplyInput struct {
	SambaBoxConfig *SambaBoxConfig `json:"sambaBoxConfig,omitempty" yaml:"sambaBoxConfig,omitempty"`
	Enabled        bool            `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Password       string          `json:"password,omitempty" yaml:"password,omitempty"`
	Username       string          `json:"username,omitempty" yaml:"username,omitempty"`
}
