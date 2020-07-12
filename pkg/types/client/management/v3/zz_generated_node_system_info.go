package client

const (
	NodeSystemInfoType                         = "nodeSystemInfo"
	NodeSystemInfoFieldArchitecture            = "architecture"
	NodeSystemInfoFieldBootID                  = "bootID"
	NodeSystemInfoFieldContainerRuntimeVersion = "containerRuntimeVersion"
	NodeSystemInfoFieldKernelVersion           = "kernelVersion"
	NodeSystemInfoFieldKubeProxyVersion        = "kubeProxyVersion"
	NodeSystemInfoFieldKubeletVersion          = "kubeletVersion"
	NodeSystemInfoFieldMachineID               = "machineID"
	NodeSystemInfoFieldOSImage                 = "osImage"
	NodeSystemInfoFieldOperatingSystem         = "operatingSystem"
	NodeSystemInfoFieldSystemUUID              = "systemUUID"
)

type NodeSystemInfo struct {
	Architecture            string `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	BootID                  string `json:"bootID,omitempty" yaml:"bootID,omitempty"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion,omitempty" yaml:"containerRuntimeVersion,omitempty"`
	KernelVersion           string `json:"kernelVersion,omitempty" yaml:"kernelVersion,omitempty"`
	KubeProxyVersion        string `json:"kubeProxyVersion,omitempty" yaml:"kubeProxyVersion,omitempty"`
	KubeletVersion          string `json:"kubeletVersion,omitempty" yaml:"kubeletVersion,omitempty"`
	MachineID               string `json:"machineID,omitempty" yaml:"machineID,omitempty"`
	OSImage                 string `json:"osImage,omitempty" yaml:"osImage,omitempty"`
	OperatingSystem         string `json:"operatingSystem,omitempty" yaml:"operatingSystem,omitempty"`
	SystemUUID              string `json:"systemUUID,omitempty" yaml:"systemUUID,omitempty"`
}
