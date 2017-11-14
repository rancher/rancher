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
	Architecture            string `json:"architecture,omitempty"`
	BootID                  string `json:"bootID,omitempty"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion,omitempty"`
	KernelVersion           string `json:"kernelVersion,omitempty"`
	KubeProxyVersion        string `json:"kubeProxyVersion,omitempty"`
	KubeletVersion          string `json:"kubeletVersion,omitempty"`
	MachineID               string `json:"machineID,omitempty"`
	OSImage                 string `json:"osImage,omitempty"`
	OperatingSystem         string `json:"operatingSystem,omitempty"`
	SystemUUID              string `json:"systemUUID,omitempty"`
}
