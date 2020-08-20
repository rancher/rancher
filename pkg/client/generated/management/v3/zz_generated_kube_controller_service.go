package client

const (
	KubeControllerServiceType                       = "kubeControllerService"
	KubeControllerServiceFieldClusterCIDR           = "clusterCidr"
	KubeControllerServiceFieldExtraArgs             = "extraArgs"
	KubeControllerServiceFieldExtraBinds            = "extraBinds"
	KubeControllerServiceFieldExtraEnv              = "extraEnv"
	KubeControllerServiceFieldImage                 = "image"
	KubeControllerServiceFieldServiceClusterIPRange = "serviceClusterIpRange"
	KubeControllerServiceFieldWindowsExtraArgs      = "winExtraArgs"
	KubeControllerServiceFieldWindowsExtraBinds     = "winExtraBinds"
	KubeControllerServiceFieldWindowsExtraEnv       = "winExtraEnv"
)

type KubeControllerService struct {
	ClusterCIDR           string            `json:"clusterCidr,omitempty" yaml:"clusterCidr,omitempty"`
	ExtraArgs             map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds            []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv              []string          `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	Image                 string            `json:"image,omitempty" yaml:"image,omitempty"`
	ServiceClusterIPRange string            `json:"serviceClusterIpRange,omitempty" yaml:"serviceClusterIpRange,omitempty"`
	WindowsExtraArgs      map[string]string `json:"winExtraArgs,omitempty" yaml:"winExtraArgs,omitempty"`
	WindowsExtraBinds     []string          `json:"winExtraBinds,omitempty" yaml:"winExtraBinds,omitempty"`
	WindowsExtraEnv       []string          `json:"winExtraEnv,omitempty" yaml:"winExtraEnv,omitempty"`
}
