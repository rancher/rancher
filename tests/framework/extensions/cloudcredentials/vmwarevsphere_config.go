package cloudcredentials

const VSphereCredentialConfigurationFileKey = "vSphereCredentials"

type VSphereCredentialConfig struct {
	ClusterId         string `json:"clusterId" yaml:"clusterId"`
	ClusterType       string `json:"clusterType" yaml:"clusterType"`
	KubeconfigContent string `json:"kubeconfigContent" yaml:"kubeconfigContent"`
}
