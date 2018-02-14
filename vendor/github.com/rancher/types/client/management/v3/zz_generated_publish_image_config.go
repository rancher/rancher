package client

const (
	PublishImageConfigType                = "publishImageConfig"
	PublishImageConfigFieldBuildContext   = "buildContext"
	PublishImageConfigFieldDockerfilePath = "dockerfilePath"
	PublishImageConfigFieldTag            = "tag"
)

type PublishImageConfig struct {
	BuildContext   string `json:"buildContext,omitempty"`
	DockerfilePath string `json:"dockerfilePath,omitempty"`
	Tag            string `json:"tag,omitempty"`
}
