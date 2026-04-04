package client

const (
	ClusterInfoType                     = "clusterInfo"
	ClusterInfoFieldArch                = "arch"
	ClusterInfoFieldKubernetesVersion   = "kubernetesVersion"
	ClusterInfoFieldMachineProvider     = "machineProvider"
	ClusterInfoFieldNodeCount           = "nodeCount"
	ClusterInfoFieldProvisioningCluster = "provisioningCluster"
)

type ClusterInfo struct {
	Arch                string `json:"arch,omitempty" yaml:"arch,omitempty"`
	KubernetesVersion   string `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	MachineProvider     string `json:"machineProvider,omitempty" yaml:"machineProvider,omitempty"`
	NodeCount           int64  `json:"nodeCount,omitempty" yaml:"nodeCount,omitempty"`
	ProvisioningCluster string `json:"provisioningCluster,omitempty" yaml:"provisioningCluster,omitempty"`
}
