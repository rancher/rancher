package client

const (
	KubeproxyServiceType            = "kubeproxyService"
	KubeproxyServiceFieldExtraArgs  = "extraArgs"
	KubeproxyServiceFieldExtraBinds = "extraBinds"
	KubeproxyServiceFieldImage      = "image"
)

type KubeproxyService struct {
	ExtraArgs  map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	Image      string            `json:"image,omitempty" yaml:"image,omitempty"`
}
