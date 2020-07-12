package client

const (
	SELinuxOptionsType       = "seLinuxOptions"
	SELinuxOptionsFieldLevel = "level"
	SELinuxOptionsFieldRole  = "role"
	SELinuxOptionsFieldType  = "type"
	SELinuxOptionsFieldUser  = "user"
)

type SELinuxOptions struct {
	Level string `json:"level,omitempty" yaml:"level,omitempty"`
	Role  string `json:"role,omitempty" yaml:"role,omitempty"`
	Type  string `json:"type,omitempty" yaml:"type,omitempty"`
	User  string `json:"user,omitempty" yaml:"user,omitempty"`
}
