package client

const (
	SELinuxOptionsType       = "seLinuxOptions"
	SELinuxOptionsFieldLevel = "level"
	SELinuxOptionsFieldRole  = "role"
	SELinuxOptionsFieldType  = "type"
	SELinuxOptionsFieldUser  = "user"
)

type SELinuxOptions struct {
	Level string `json:"level,omitempty"`
	Role  string `json:"role,omitempty"`
	Type  string `json:"type,omitempty"`
	User  string `json:"user,omitempty"`
}
