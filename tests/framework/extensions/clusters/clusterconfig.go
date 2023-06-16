package clusters

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	provisioningInput "github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
)

type ClusterConfig struct {
	KubernetesVersion    string                                   `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNI                  string                                   `json:"cni" yaml:"cni"`
	PSACT                string                                   `json:"psact" yaml:"psact"`
	PNI                  bool                                     `json:"pni" yaml:"pni"`
	NodesAndRoles        *[]machinepools.NodeRoles                `json:"nodesAndRoles" yaml:"nodesAndRoles" default:"[]"`
	NodesAndRolesRKE1    *[]nodepools.NodeRoles                   `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1" default:"[]"`
	Providers            *[]string                                `json:"providers" yaml:"providers"`
	NodeProviders        *[]string                                `json:"nodeProviders" yaml:"nodeProviders"`
	Hardened             bool                                     `json:"hardened" yaml:"hardened"`
	AddOnConfig          *provisioningInput.AddOnConfig           `json:"addonConfig" yaml:"addonConfig"`
	AgentEnvVars         *[]rkev1.EnvVar                          `json:"agentEnvVars" yaml:"agentEnvVars"`
	AgentEnvVarsRKE1     *[]management.EnvVar                     `json:"agentEnvVarsRKE1" yaml:"agentEnvVarsRKE1"`
	ClusterAgent         *management.AgentDeploymentCustomization `json:"clusterAgent" yaml:"clusterAgent"`
	FleetAgent           *management.AgentDeploymentCustomization `json:"fleetAgent" yaml:"fleetAgent"`
	Etcd                 *rkev1.ETCD                              `json:"etcd" yaml:"etcd"`
	LabelsAndAnnotations *provisioningInput.LabelsAndAnnotations  `json:"labelsAndAnnotations" yaml:"labelsAndAnnotations"`
	Networking           *provisioningInput.Networking            `json:"networking" yaml:"networking"`
	Registries           *provisioningInput.Registries            `json:"registries" yaml:"registries"`
	UpgradeStrategy      *rkev1.ClusterUpgradeStrategy            `json:"upgradeStrategy" yaml:"upgradeStrategy"`
	Advanced             *provisioningInput.Advanced              `json:"advanced" yaml:"advanced"`
}

// ConvertConfigToClusterConfig converts the config from (user) provisioning input to a cluster config
func ConvertConfigToClusterConfig(clustersConfig *provisioningInput.Config) *ClusterConfig {
	var newConfig ClusterConfig
	newConfig.AddOnConfig = clustersConfig.AddOnConfig
	newConfig.NodesAndRoles = &clustersConfig.NodesAndRoles
	newConfig.NodesAndRolesRKE1 = &clustersConfig.NodesAndRolesRKE1
	newConfig.AgentEnvVars = clustersConfig.AgentEnvVars
	newConfig.Networking = clustersConfig.Networking
	newConfig.Advanced = clustersConfig.Advanced
	newConfig.Providers = &clustersConfig.Providers
	newConfig.NodeProviders = &clustersConfig.NodeProviders
	newConfig.ClusterAgent = clustersConfig.ClusterAgent
	newConfig.FleetAgent = clustersConfig.FleetAgent
	newConfig.Etcd = clustersConfig.Etcd
	newConfig.LabelsAndAnnotations = clustersConfig.LabelsAndAnnotations
	newConfig.Registries = clustersConfig.Registries
	newConfig.UpgradeStrategy = clustersConfig.UpgradeStrategy

	newConfig.Hardened = clustersConfig.Hardened
	newConfig.PSACT = clustersConfig.PSACT
	newConfig.PNI = clustersConfig.PNI

	return &newConfig
}
