package client

const (
	KubeAPIServiceType                       = "kubeAPIService"
	KubeAPIServiceFieldExtraArgs             = "extraArgs"
	KubeAPIServiceFieldImage                 = "image"
	KubeAPIServiceFieldServiceClusterIPRange = "serviceClusterIpRange"
)

type KubeAPIService struct {
	ExtraArgs             map[string]string `json:"extraArgs,omitempty"`
	Image                 string            `json:"image,omitempty"`
	ServiceClusterIPRange string            `json:"serviceClusterIpRange,omitempty"`
}
