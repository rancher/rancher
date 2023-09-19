package cloudcredentials

// The json/yaml config key for the harvester cloud credential config
const HarvesterCredentialConfigurationFileKey = "harvesterCredentials"

// HarvesterCredentialConfig is configuration need to create a harvester cloud credential
type HarvesterCredentialConfig struct {
	ClusterID         string `json:"clusterId" yaml:"clusterId"`
	ClusterType       string `json:"clusterType" yaml:"clusterType"`
	KubeconfigContent string `json:"kubeconfigContent" yaml:"kubeconfigContent"`
}
