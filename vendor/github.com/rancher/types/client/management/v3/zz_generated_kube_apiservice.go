package client

const (
	KubeAPIServiceType                       = "kubeAPIService"
	KubeAPIServiceFieldExtraArgs             = "extraArgs"
	KubeAPIServiceFieldImage                 = "image"
	KubeAPIServiceFieldPodSecurityPolicy     = "podSecurityPolicy"
	KubeAPIServiceFieldServiceClusterIPRange = "serviceClusterIpRange"
)

type KubeAPIService struct {
	ExtraArgs             map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	Image                 string            `json:"image,omitempty" yaml:"image,omitempty"`
	PodSecurityPolicy     bool              `json:"podSecurityPolicy,omitempty" yaml:"podSecurityPolicy,omitempty"`
	ServiceClusterIPRange string            `json:"serviceClusterIpRange,omitempty" yaml:"serviceClusterIpRange,omitempty"`
}
