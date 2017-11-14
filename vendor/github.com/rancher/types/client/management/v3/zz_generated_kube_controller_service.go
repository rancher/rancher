package client

const (
	KubeControllerServiceType                       = "kubeControllerService"
	KubeControllerServiceFieldClusterCIDR           = "clusterCidr"
	KubeControllerServiceFieldExtraArgs             = "extraArgs"
	KubeControllerServiceFieldImage                 = "image"
	KubeControllerServiceFieldServiceClusterIPRange = "serviceClusterIpRange"
)

type KubeControllerService struct {
	ClusterCIDR           string            `json:"clusterCidr,omitempty"`
	ExtraArgs             map[string]string `json:"extraArgs,omitempty"`
	Image                 string            `json:"image,omitempty"`
	ServiceClusterIPRange string            `json:"serviceClusterIpRange,omitempty"`
}
