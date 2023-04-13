package client

const (
	LaunchTemplateType         = "launchTemplate"
	LaunchTemplateFieldID      = "id"
	LaunchTemplateFieldName    = "name"
	LaunchTemplateFieldVersion = "version"
)

type LaunchTemplate struct {
	ID      *string `json:"id,omitempty" yaml:"id,omitempty"`
	Name    *string `json:"name,omitempty" yaml:"name,omitempty"`
	Version *int64  `json:"version,omitempty" yaml:"version,omitempty"`
}
