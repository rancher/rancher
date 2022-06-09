package client

const (
	KubeletServiceType                            = "kubeletService"
	KubeletServiceFieldClusterDNSServer           = "clusterDnsServer"
	KubeletServiceFieldClusterDomain              = "clusterDomain"
	KubeletServiceFieldExtraArgs                  = "extraArgs"
	KubeletServiceFieldExtraBinds                 = "extraBinds"
	KubeletServiceFieldExtraEnv                   = "extraEnv"
	KubeletServiceFieldFailSwapOn                 = "failSwapOn"
	KubeletServiceFieldGenerateServingCertificate = "generateServingCertificate"
	KubeletServiceFieldImage                      = "image"
	KubeletServiceFieldInfraContainerImage        = "infraContainerImage"
	KubeletServiceFieldWindowsExtraArgs           = "winExtraArgs"
	KubeletServiceFieldWindowsExtraBinds          = "winExtraBinds"
	KubeletServiceFieldWindowsExtraEnv            = "winExtraEnv"
)

type KubeletService struct {
	ClusterDNSServer           string            `json:"clusterDnsServer,omitempty" yaml:"clusterDnsServer,omitempty"`
	ClusterDomain              string            `json:"clusterDomain,omitempty" yaml:"clusterDomain,omitempty"`
	ExtraArgs                  map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ExtraBinds                 []string          `json:"extraBinds,omitempty" yaml:"extraBinds,omitempty"`
	ExtraEnv                   []string          `json:"extraEnv,omitempty" yaml:"extraEnv,omitempty"`
	FailSwapOn                 bool              `json:"failSwapOn,omitempty" yaml:"failSwapOn,omitempty"`
	GenerateServingCertificate bool              `json:"generateServingCertificate,omitempty" yaml:"generateServingCertificate,omitempty"`
	Image                      string            `json:"image,omitempty" yaml:"image,omitempty"`
	InfraContainerImage        string            `json:"infraContainerImage,omitempty" yaml:"infraContainerImage,omitempty"`
	WindowsExtraArgs           map[string]string `json:"winExtraArgs,omitempty" yaml:"winExtraArgs,omitempty"`
	WindowsExtraBinds          []string          `json:"winExtraBinds,omitempty" yaml:"winExtraBinds,omitempty"`
	WindowsExtraEnv            []string          `json:"winExtraEnv,omitempty" yaml:"winExtraEnv,omitempty"`
}
