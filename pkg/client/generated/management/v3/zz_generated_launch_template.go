package client

const (
	LaunchTemplateType         = "launchTemplate"
	LaunchTemplateFieldName    = "name"
	LaunchTemplateFieldVersion = "version"
)

type LaunchTemplate struct {
	Name    *string `json:"name,omitempty" yaml:"name,omitempty"`
	Version *int64  `json:"version,omitempty" yaml:"version,omitempty"`
}
