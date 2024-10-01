package clusters

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	provisioningInput "github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
)

type ClusterConfig struct {
	KubernetesVersion              string                                            `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNI                            string                                            `json:"cni" yaml:"cni"`
	PSACT                          string                                            `json:"psact" yaml:"psact"`
	PNI                            bool                                              `json:"pni" yaml:"pni"`
	NodePools                      []provisioningInput.NodePools                     `json:"nodepools" yaml:"nodepools"`
	MachinePools                   []provisioningInput.MachinePools                  `json:"machinepools" yaml:"machinepools"`
	CloudProvider                  string                                            `json:"cloudProvider" yaml:"cloudProvider"`
	Providers                      *[]string                                         `json:"providers" yaml:"providers"`
	NodeProviders                  *[]string                                         `json:"nodeProviders" yaml:"nodeProviders"`
	Hardened                       bool                                              `json:"hardened" yaml:"hardened"`
	AddOnConfig                    *provisioningInput.AddOnConfig                    `json:"addonConfig" yaml:"addonConfig"`
	AgentEnvVars                   []rkev1.EnvVar                                    `json:"agentEnvVars" yaml:"agentEnvVars"`
	AgentEnvVarsRKE1               []management.EnvVar                               `json:"agentEnvVarsRKE1" yaml:"agentEnvVarsRKE1"`
	ClusterAgent                   *management.AgentDeploymentCustomization          `json:"clusterAgent" yaml:"clusterAgent"`
	FleetAgent                     *management.AgentDeploymentCustomization          `json:"fleetAgent" yaml:"fleetAgent"`
	ETCD                           *rkev1.ETCD                                       `json:"etcd" yaml:"etcd"`
	ETCDRKE1                       *management.ETCDService                           `json:"etcdRKE1" yaml:"etcdRKE1"`
	LabelsAndAnnotations           *provisioningInput.LabelsAndAnnotations           `json:"labelsAndAnnotations" yaml:"labelsAndAnnotations"`
	Networking                     *provisioningInput.Networking                     `json:"networking" yaml:"networking"`
	Registries                     *provisioningInput.Registries                     `json:"registries" yaml:"registries"`
	UpgradeStrategy                *rkev1.ClusterUpgradeStrategy                     `json:"upgradeStrategy" yaml:"upgradeStrategy"`
	Advanced                       *provisioningInput.Advanced                       `json:"advanced" yaml:"advanced"`
	ClusterSSHTests                []provisioningInput.SSHTestCase                   `json:"clusterSSHTests" yaml:"clusterSSHTests"`
	CRIDockerd                     bool                                              `json:"criDockerd" yaml:"criDockerd"`
	RKE1CustomClusterDockerInstall *provisioningInput.RKE1CustomClusterDockerInstall `json:"rke1CustomClusterDockerInstall" yaml:"rke1CustomClusterDockerInstall"`
}

// ConvertConfigToClusterConfig converts the config from (user) provisioning input to a cluster config
func ConvertConfigToClusterConfig(provisioningConfig *provisioningInput.Config) *ClusterConfig {
	var newConfig ClusterConfig
	newConfig.AddOnConfig = provisioningConfig.AddOnConfig
	newConfig.MachinePools = provisioningConfig.MachinePools
	newConfig.NodePools = provisioningConfig.NodePools
	newConfig.AgentEnvVars = provisioningConfig.AgentEnvVars
	newConfig.AgentEnvVarsRKE1 = provisioningConfig.AgentEnvVarsRKE1
	newConfig.Networking = provisioningConfig.Networking
	newConfig.Advanced = provisioningConfig.Advanced
	newConfig.Providers = &provisioningConfig.Providers
	newConfig.NodeProviders = &provisioningConfig.NodeProviders
	newConfig.ClusterAgent = provisioningConfig.ClusterAgent
	newConfig.FleetAgent = provisioningConfig.FleetAgent
	newConfig.ETCD = provisioningConfig.ETCD
	newConfig.ETCDRKE1 = provisioningConfig.ETCDRKE1
	newConfig.LabelsAndAnnotations = provisioningConfig.LabelsAndAnnotations
	newConfig.Registries = provisioningConfig.Registries
	newConfig.UpgradeStrategy = provisioningConfig.UpgradeStrategy
	newConfig.CloudProvider = provisioningConfig.CloudProvider

	newConfig.Hardened = provisioningConfig.Hardened
	newConfig.PSACT = provisioningConfig.PSACT
	newConfig.PNI = provisioningConfig.PNI
	newConfig.ClusterSSHTests = provisioningConfig.ClusterSSHTests
	newConfig.RKE1CustomClusterDockerInstall = provisioningConfig.RKE1CustomClusterDockerInstall

	return &newConfig
}
