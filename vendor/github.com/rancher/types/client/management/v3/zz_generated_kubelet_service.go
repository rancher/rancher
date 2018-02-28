package client

const (
	KubeletServiceType                     = "kubeletService"
	KubeletServiceFieldClusterDNSServer    = "clusterDnsServer"
	KubeletServiceFieldClusterDomain       = "clusterDomain"
	KubeletServiceFieldExtraArgs           = "extraArgs"
	KubeletServiceFieldFailSwapOn          = "failSwapOn"
	KubeletServiceFieldImage               = "image"
	KubeletServiceFieldInfraContainerImage = "infraContainerImage"
)

type KubeletService struct {
	ClusterDNSServer    string            `json:"clusterDnsServer,omitempty" yaml:"clusterDnsServer,omitempty"`
	ClusterDomain       string            `json:"clusterDomain,omitempty" yaml:"clusterDomain,omitempty"`
	ExtraArgs           map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	FailSwapOn          bool              `json:"failSwapOn,omitempty" yaml:"failSwapOn,omitempty"`
	Image               string            `json:"image,omitempty" yaml:"image,omitempty"`
	InfraContainerImage string            `json:"infraContainerImage,omitempty" yaml:"infraContainerImage,omitempty"`
}
