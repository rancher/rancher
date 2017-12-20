package client

const (
	KubeAPIServiceType                       = "kubeAPIService"
	KubeAPIServiceFieldExtraArgs             = "extraArgs"
	KubeAPIServiceFieldImage                 = "image"
	KubeAPIServiceFieldPodSecurityPolicy     = "podSecurityPolicy"
	KubeAPIServiceFieldServiceClusterIPRange = "serviceClusterIpRange"
)

type KubeAPIService struct {
	ExtraArgs             map[string]string `json:"extraArgs,omitempty"`
	Image                 string            `json:"image,omitempty"`
	PodSecurityPolicy     *bool             `json:"podSecurityPolicy,omitempty"`
	ServiceClusterIPRange string            `json:"serviceClusterIpRange,omitempty"`
}
