package nodetemplates

// The json/yaml config key for the Harvester node template config
const HarvesterNodeTemplateConfigurationFileKey = "harvesterConfig"

// HarvesterNodeTemplateConfig is configuration need to create a Harvester node template
type HarvesterNodeTemplateConfig struct {
	CloudConfig       string `json:"cloudConfig" yaml:"cloudConfig"`
	ClusterID         string `json:"clusterId" yaml:"clusterId"`
	ClusterType       string `json:"clusterType" yaml:"clusterType"`
	CPUCount          string `json:"cpuCount" yaml:"cpuCount"`
	DiskBus           string `json:"diskBus" yaml:"diskBus"`
	DiskSize          string `json:"diskSize" yaml:"diskSize"`
	ImageName         string `json:"imageName" yaml:"imageName"`
	KeyPairName       string `json:"keyPairName" yaml:"keyPairName"`
	KubeconfigContent string `json:"kubeconfigContent" yaml:"kubeconfigContent"`
	MemorySize        string `json:"memorySize" yaml:"memorySize"`
	NetworkData       string `json:"networkData" yaml:"networkData"`
	NetworkModel      string `json:"networkModel" yaml:"networkModel"`
	NetworkName       string `json:"networkName" yaml:"networkName"`
	NetworkType       string `json:"networkType" yaml:"networkType"`
	SSHPassword       string `json:"sshPassword" yaml:"sshPassword"`
	SSHPort           string `json:"sshPort" yaml:"sshPort"`
	SSHPrivateKeyPath string `json:"sshPrivateKeyPath" yaml:"sshPrivateKeyPath"`
	SSHUser           string `json:"sshUser" yaml:"sshUser"`
	Type              string `json:"type" yaml:"type"`
	UserData          string `json:"userData" yaml:"userData"`
	VMAffinity        string `json:"vmAffinity" yaml:"vmAffinity"`
	VMNamespace       string `json:"vmNamespace" yaml:"vmNamespace"`
}
