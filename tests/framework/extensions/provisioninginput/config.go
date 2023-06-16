package provisioninginput

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
)

type Version string
type PSACT string

const (
	Namespace                       = "fleet-default"
	defaultRandStringLength         = 5
	ConfigurationFileKey            = "provisioningInput"
	HardenedKubeVersion     Version = "v1.24.99"
	RancherPrivileged       PSACT   = "rancher-privileged"
	RancherRestricted       PSACT   = "rancher-restricted"
)

// ProviderName is string enum for provider names used in provisioning tests.
type ProviderName string

const (
	AWSProviderName       ProviderName = "aws"
	AzureProviderName     ProviderName = "azure"
	DOProviderName        ProviderName = "do"
	HarvesterProviderName ProviderName = "harvester"
	LinodeProviderName    ProviderName = "linode"
	GoogleProviderName    ProviderName = "google"
	VsphereProviderName   ProviderName = "vsphere"
)

var AllRolesPool = machinepools.NodeRoles{
	Etcd:         true,
	ControlPlane: true,
	Worker:       true,
	Quantity:     1,
}

var EtcdControlPlanePool = machinepools.NodeRoles{
	Etcd:         true,
	ControlPlane: true,
	Quantity:     1,
}

var EtcdPool = machinepools.NodeRoles{
	Etcd:     true,
	Quantity: 1,
}

var ControlPlanePool = machinepools.NodeRoles{
	ControlPlane: true,
	Quantity:     1,
}

var WorkerPool = machinepools.NodeRoles{
	Worker:   true,
	Quantity: 1,
}

var WindowsPool = machinepools.NodeRoles{
	Windows:  true,
	Quantity: 1,
}

var RKE1AllRolesPool = nodepools.NodeRoles{
	Etcd:         true,
	ControlPlane: true,
	Worker:       true,
	Quantity:     1,
}

var RKE1EtcdControlPlanePool = nodepools.NodeRoles{
	Etcd:         true,
	ControlPlane: true,
	Quantity:     1,
}

var RKE1EtcdPool = nodepools.NodeRoles{
	Etcd:     true,
	Quantity: 1,
}

var RKE1ControlPlanePool = nodepools.NodeRoles{
	ControlPlane: true,
	Quantity:     1,
}

var RKE1WorkerPool = nodepools.NodeRoles{
	Worker:   true,
	Quantity: 1,
}

// String stringer for the ProviderName
func (p ProviderName) String() string {
	return string(p)
}

// TestClientName is string enum for client/user names used in provisioning tests.
type TestClientName string

const (
	AdminClientName    TestClientName = "Admin User"
	StandardClientName TestClientName = "Standard User"
)

// String stringer for the TestClientName
func (c TestClientName) String() string {
	return string(c)
}

type AddOnConfig struct {
	ChartValues        *rkev1.GenericMap `json:"chartValues" yaml:"chartValues"`
	AdditionalManifest string            `json:"additionalManifest" yaml:"additionalManifest"`
}

type LabelsAndAnnotations struct {
	Labels      map[string]string `json:"labels" yaml:"labels"`
	Annotations map[string]string `json:"annotations" yaml:"annotations"`
}

type Networking struct {
	ClusterCIDR              string                          `json:"clusterCIDR" yaml:"clusterCIDR"`
	ClusterDNS               string                          `json:"clusterDNS" yaml:"clusterDNS"`
	ClusterDomain            string                          `json:"clusterDomain" yaml:"clusterDomain"`
	ServiceCIDR              string                          `json:"serviceCIDR" yaml:"serviceCIDR"`
	NodePortServicePortRange string                          `json:"nodePortServicePortRange" yaml:"nodePortServicePortRange"`
	TLSSan                   []string                        `json:"tlsSan" yaml:"tlsSan"`
	LocalClusterAuthEndpoint *rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint" yaml:"localClusterAuthEndpoint"`
}

type Advanced struct {
	// examples of machineSelector configs: "protect-kernel-defaults": false, "system-default-registry": registryHostname,
	MachineSelectors          *[]rkev1.RKESystemConfig `json:"machineSelectors" yaml:"machineSelectors"`
	MachineGlobalConfig       *rkev1.GenericMap        `json:"machineGlobalConfig" yaml:"machineGlobalConfig"`
	KubeControllerManagerArgs []string                 `json:"kubeControllerManagerArgs" yaml:"kubeControllerManagerArgs"`
	KubeSchedulerArgs         []string                 `json:"kubeSchedulerArgs" yaml:"kubeSchedulerArgs"`
	KubeAPIServerArgs         []string                 `json:"kubeAPIServerArgs" yaml:"kubeAPIServerArgs"`
}

type Registries struct {
	RKE1Registries []management.PrivateRegistry `json:"rke1Registries" yaml:"rke1Registries"`
	RKE2Registries *rkev1.Registry              `json:"rke2Registries" yaml:"rke2Registries"`
	RKE2Password   string                       `json:"rke2Password" yaml:"rke2Password"`
	RKE2Username   string                       `json:"rke2Username" yaml:"rke2Username"`
}

type Config struct {
	NodesAndRoles          []machinepools.NodeRoles                 `json:"nodesAndRoles" yaml:"nodesAndRoles" default:"[]"`
	NodesAndRolesRKE1      []nodepools.NodeRoles                    `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1" default:"[]"`
	Providers              []string                                 `json:"providers" yaml:"providers"`
	NodeProviders          []string                                 `json:"nodeProviders" yaml:"nodeProviders"`
	Hardened               bool                                     `json:"hardened" yaml:"hardened"`
	AddOnConfig            *AddOnConfig                             `json:"addonConfig" yaml:"addonConfig"`
	K3SKubernetesVersions  []string                                 `json:"k3sKubernetesVersion" yaml:"k3sKubernetesVersion"`
	RKE1KubernetesVersions []string                                 `json:"rke1KubernetesVersion" yaml:"rke1KubernetesVersion"`
	RKE2KubernetesVersions []string                                 `json:"rke2KubernetesVersion" yaml:"rke2KubernetesVersion"`
	CNIs                   []string                                 `json:"cni" yaml:"cni"`
	PSACT                  string                                   `json:"psact" yaml:"psact"`
	PNI                    bool                                     `json:"pni" yaml:"pni"`
	AgentEnvVars           *[]rkev1.EnvVar                          `json:"agentEnvVars" yaml:"agentEnvVars"`
	AgentEnvVarsRKE1       *[]management.EnvVar                     `json:"agentEnvVarsRKE1" yaml:"agentEnvVarsRKE1"`
	ClusterAgent           *management.AgentDeploymentCustomization `json:"clusterAgent" yaml:"clusterAgent"`
	FleetAgent             *management.AgentDeploymentCustomization `json:"fleetAgent" yaml:"fleetAgent"`
	Etcd                   *rkev1.ETCD                              `json:"etcd" yaml:"etcd"`
	LabelsAndAnnotations   *LabelsAndAnnotations                    `json:"labelsAndAnnotations" yaml:"labelsAndAnnotations"`
	Networking             *Networking                              `json:"networking" yaml:"networking"`
	Registries             *Registries                              `json:"registries" yaml:"registries"`
	UpgradeStrategy        *rkev1.ClusterUpgradeStrategy            `json:"upgradeStrategy" yaml:"upgradeStrategy"`
	Advanced               *Advanced                                `json:"advanced" yaml:"advanced"`
}
