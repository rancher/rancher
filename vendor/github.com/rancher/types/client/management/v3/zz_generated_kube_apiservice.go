package client

const (
	KubeAPIServiceType                         = "kubeAPIService"
	KubeAPIServiceFieldAlwaysPullImages        = "alwaysPullImages"
	KubeAPIServiceFieldExtraArgs               = "extraArgs"
	KubeAPIServiceFieldExtraBinds              = "extraBinds"
	KubeAPIServiceFieldExtraEnv                = "extraEnv"
	KubeAPIServiceFieldImage                   = "image"
	KubeAPIServiceFieldPodSecurityPolicy       = "podSecurityPolicy"
	KubeAPIServiceFieldSecretsEncryptionConfig = "secretsEncryptionConfig"
	KubeAPIServiceFieldServiceClusterIPRange   = "serviceClusterIpRange"
	KubeAPIServiceFieldServiceNodePortRange    = "serviceNodePortRange"
)

type KubeAPIService struct {
	AlwaysPullImages        bool                     `json:"alwaysPullImages,omitempty" yaml:"alwaysPullImages,omitempty"`
	ExtraArgs               map[string]string        `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds              []string                 `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv                []string                 `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	Image                   string                   `json:"image,omitempty" yaml:"image,omitempty"`
	PodSecurityPolicy       bool                     `json:"podSecurityPolicy,omitempty" yaml:"podSecurityPolicy,omitempty"`
	SecretsEncryptionConfig *SecretsEncryptionConfig `json:"secretsEncryptionConfig,omitempty" yaml:"secretsEncryptionConfig,omitempty"`
	ServiceClusterIPRange   string                   `json:"serviceClusterIpRange,omitempty" yaml:"serviceClusterIpRange,omitempty"`
	ServiceNodePortRange    string                   `json:"serviceNodePortRange,omitempty" yaml:"serviceNodePortRange,omitempty"`
}
