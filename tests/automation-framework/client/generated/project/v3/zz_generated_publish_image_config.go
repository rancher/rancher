package client

const (
	PublishImageConfigType                = "publishImageConfig"
	PublishImageConfigFieldBuildContext   = "buildContext"
	PublishImageConfigFieldDockerfilePath = "dockerfilePath"
	PublishImageConfigFieldPushRemote     = "pushRemote"
	PublishImageConfigFieldRegistry       = "registry"
	PublishImageConfigFieldTag            = "tag"
)

type PublishImageConfig struct {
	BuildContext   string `json:"buildContext,omitempty" yaml:"buildContext,omitempty"`
	DockerfilePath string `json:"dockerfilePath,omitempty" yaml:"dockerfilePath,omitempty"`
	PushRemote     bool   `json:"pushRemote,omitempty" yaml:"pushRemote,omitempty"`
	Registry       string `json:"registry,omitempty" yaml:"registry,omitempty"`
	Tag            string `json:"tag,omitempty" yaml:"tag,omitempty"`
}
