package client

const (
	KubeletServiceType                     = "kubeletService"
	KubeletServiceFieldClusterDNSServer    = "clusterDnsServer"
	KubeletServiceFieldClusterDomain       = "clusterDomain"
	KubeletServiceFieldExtraArgs           = "extraArgs"
	KubeletServiceFieldImage               = "image"
	KubeletServiceFieldInfraContainerImage = "infraContainerImage"
)

type KubeletService struct {
	ClusterDNSServer    string            `json:"clusterDnsServer,omitempty"`
	ClusterDomain       string            `json:"clusterDomain,omitempty"`
	ExtraArgs           map[string]string `json:"extraArgs,omitempty"`
	Image               string            `json:"image,omitempty"`
	InfraContainerImage string            `json:"infraContainerImage,omitempty"`
}
