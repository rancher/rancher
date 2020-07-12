package client

const (
	AllowedHostPathType            = "allowedHostPath"
	AllowedHostPathFieldPathPrefix = "pathPrefix"
	AllowedHostPathFieldReadOnly   = "readOnly"
)

type AllowedHostPath struct {
	PathPrefix string `json:"pathPrefix,omitempty" yaml:"pathPrefix,omitempty"`
	ReadOnly   bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}
