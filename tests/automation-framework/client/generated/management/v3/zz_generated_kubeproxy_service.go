package client

const (
	KubeproxyServiceType                   = "kubeproxyService"
	KubeproxyServiceFieldExtraArgs         = "extraArgs"
	KubeproxyServiceFieldExtraBinds        = "extraBinds"
	KubeproxyServiceFieldExtraEnv          = "extraEnv"
	KubeproxyServiceFieldImage             = "image"
	KubeproxyServiceFieldWindowsExtraArgs  = "winExtraArgs"
	KubeproxyServiceFieldWindowsExtraBinds = "winExtraBinds"
	KubeproxyServiceFieldWindowsExtraEnv   = "winExtraEnv"
)

type KubeproxyService struct {
	ExtraArgs         map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds        []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv          []string          `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	Image             string            `json:"image,omitempty" yaml:"image,omitempty"`
	WindowsExtraArgs  map[string]string `json:"winExtraArgs,omitempty" yaml:"winExtraArgs,omitempty"`
	WindowsExtraBinds []string          `json:"winExtraBinds,omitempty" yaml:"winExtraBinds,omitempty"`
	WindowsExtraEnv   []string          `json:"winExtraEnv,omitempty" yaml:"winExtraEnv,omitempty"`
}
