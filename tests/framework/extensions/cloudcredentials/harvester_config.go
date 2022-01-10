package cloudcredentials

const HarvesterCredentialConfigurationFileKey = "harvesterCredentials"

type HarvesterCredentialConfig struct {
	ClusterId         string `json:"clusterId" yaml:"clusterId"`
	ClusterType       string `json:"clusterType" yaml:"clusterType"`
	KubeconfigContent string `json:"kubeconfigContent" yaml:"kubeconfigContent"`
}
