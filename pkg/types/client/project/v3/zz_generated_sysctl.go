package client

const (
	SysctlType       = "sysctl"
	SysctlFieldName  = "name"
	SysctlFieldValue = "value"
)

type Sysctl struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}
