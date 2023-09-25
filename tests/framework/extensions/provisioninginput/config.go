package provisioninginput

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	corev1 "k8s.io/api/core/v1"
)

type Version string
type PSACT string
type SSHTestCase string

const (
	Namespace                       = "fleet-default"
	defaultRandStringLength         = 5
	ConfigurationFileKey            = "provisioningInput"
	PSPKubeVersionLimit     Version = "v1.24.99"
	RancherPrivileged       PSACT   = "rancher-privileged"
	RancherRestricted       PSACT   = "rancher-restricted"
	RancherBaseline         PSACT   = "rancher-baseline"
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
	ExternalProviderName  ProviderName = "external"
)

var AllRolesMachinePool = MachinePools{
	NodeRoles: machinepools.NodeRoles{
		Etcd:         true,
		ControlPlane: true,
		Worker:       true,
		Quantity:     1,
	},
}

var EtcdControlPlaneMachinePool = MachinePools{
	NodeRoles: machinepools.NodeRoles{
		Etcd:         true,
		ControlPlane: true,
		Quantity:     1,
	},
}

var EtcdMachinePool = MachinePools{
	NodeRoles: machinepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	},
}

var ControlPlaneMachinePool = MachinePools{
	NodeRoles: machinepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	},
}

var WorkerMachinePool = MachinePools{
	NodeRoles: machinepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	},
}

var WindowsMachinePool = MachinePools{
	NodeRoles: machinepools.NodeRoles{
		Windows:  true,
		Quantity: 1,
	},
}

var AllRolesNodePool = NodePools{
	NodeRoles: nodepools.NodeRoles{
		Etcd:         true,
		ControlPlane: true,
		Worker:       true,
		Quantity:     1,
	},
}

var EtcdControlPlaneNodePool = NodePools{
	NodeRoles: nodepools.NodeRoles{
		Etcd:         true,
		ControlPlane: true,
		Quantity:     1,
	},
}

var EtcdNodePool = NodePools{
	NodeRoles: nodepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	},
}

var ControlPlaneNodePool = NodePools{
	NodeRoles: nodepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	},
}

var WorkerNodePool = NodePools{
	NodeRoles: nodepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	},
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
	ChartValues        *rkev1.GenericMap `json:"chartValues,omitempty" yaml:"chartValues,omitempty"`
	AdditionalManifest string            `json:"additionalManifest,omitempty" yaml:"additionalManifest,omitempty"`
}

type LabelsAndAnnotations struct {
	Labels      map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type Networking struct {
	ClusterCIDR              string                          `json:"clusterCIDR,omitempty" yaml:"clusterCIDR,omitempty"`
	ClusterDNS               string                          `json:"clusterDNS,omitempty" yaml:"clusterDNS,omitempty"`
	ClusterDomain            string                          `json:"clusterDomain,omitempty" yaml:"clusterDomain,omitempty"`
	ServiceCIDR              string                          `json:"serviceCIDR,omitempty" yaml:"serviceCIDR,omitempty"`
	NodePortServicePortRange string                          `json:"nodePortServicePortRange,omitempty" yaml:"nodePortServicePortRange,omitempty"`
	TLSSan                   []string                        `json:"tlsSan,omitempty" yaml:"tlsSan,omitempty"`
	LocalClusterAuthEndpoint *rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty" yaml:"localClusterAuthEndpoint,omitempty"`
}

type Advanced struct {
	// examples of machineSelector configs: "protect-kernel-defaults": false, "system-default-registry": registryHostname,
	MachineSelectors          *[]rkev1.RKESystemConfig `json:"machineSelectors,omitempty" yaml:"machineSelectors,omitempty"`
	MachineGlobalConfig       *rkev1.GenericMap        `json:"machineGlobalConfig,omitempty" yaml:"machineGlobalConfig,omitempty"`
	KubeControllerManagerArgs []string                 `json:"kubeControllerManagerArgs,omitempty" yaml:"kubeControllerManagerArgs,omitempty"`
	KubeSchedulerArgs         []string                 `json:"kubeSchedulerArgs,omitempty" yaml:"kubeSchedulerArgs,omitempty"`
	KubeAPIServerArgs         []string                 `json:"kubeAPIServerArgs,omitempty" yaml:"kubeAPIServerArgs,omitempty"`
}

type Registries struct {
	RKE1Registries []management.PrivateRegistry `json:"rke1Registries,omitempty" yaml:"rke1Registries,omitempty"`
	RKE2Registries *rkev1.Registry              `json:"rke2Registries,omitempty" yaml:"rke2Registries,omitempty"`
	RKE2Password   string                       `json:"rke2Password,omitempty" yaml:"rke2Password,omitempty"`
	RKE2Username   string                       `json:"rke2Username,omitempty" yaml:"rke2Username,omitempty"`
}

type Pools struct {
	NodeLabels             map[string]string `json:"nodeLabels,omitempty" yaml:"nodeLabels,omitempty"`
	NodeTaints             []corev1.Taint    `json:"nodeTaints,omitempty" yaml:"nodeTaints,omitempty"`
	SpecifyCustomPrivateIP bool              `json:"specifyPrivateIP,omitempty" yaml:"specifyPrivateIP,omitempty"`
	SpecifyCustomPublicIP  bool              `json:"specifyPublicIP,omitempty" yaml:"specifyPublicIP,omitempty" default:"true"`
	CustomNodeNameSuffix   string            `json:"nodeNameSuffix,omitempty" yaml:"nodeNameSuffix,omitempty"`
}

type MachinePools struct {
	Pools
	NodeRoles machinepools.NodeRoles `json:"nodeRoles,omitempty" yaml:"nodeRoles,omitempty" default:"[]"`
	IsSecure  bool                   `json:"isSecure,omitempty" yaml:"isSecure,omitempty" default:"false"`
}

type NodePools struct {
	Pools
	NodeRoles nodepools.NodeRoles `json:"nodeRoles,omitempty" yaml:"nodeRoles,omitempty" default:"[]"`
}

type Config struct {
	NodePools              []NodePools                              `json:"nodePools,omitempty" yaml:"nodePools,omitempty"`
	MachinePools           []MachinePools                           `json:"machinePools,omitempty" yaml:"machinePools,omitempty"`
	CloudProvider          string                                   `json:"cloudProvider" yaml:"cloudProvider"`
	Providers              []string                                 `json:"providers,omitempty" yaml:"providers,omitempty"`
	NodeProviders          []string                                 `json:"nodeProviders,omitempty" yaml:"nodeProviders,omitempty"`
	Hardened               bool                                     `json:"hardened,omitempty" yaml:"hardened,omitempty"`
	AddOnConfig            *AddOnConfig                             `json:"addonConfig,omitempty" yaml:"addonConfig,omitempty"`
	K3SKubernetesVersions  []string                                 `json:"k3sKubernetesVersion,omitempty" yaml:"k3sKubernetesVersion,omitempty"`
	RKE1KubernetesVersions []string                                 `json:"rke1KubernetesVersion,omitempty" yaml:"rke1KubernetesVersion,omitempty"`
	RKE2KubernetesVersions []string                                 `json:"rke2KubernetesVersion,omitempty" yaml:"rke2KubernetesVersion,omitempty"`
	CNIs                   []string                                 `json:"cni,omitempty" yaml:"cni,omitempty"`
	PSACT                  string                                   `json:"psact,omitempty" yaml:"psact,omitempty"`
	PNI                    bool                                     `json:"pni,omitempty" yaml:"pni,omitempty"`
	AgentEnvVars           *[]rkev1.EnvVar                          `json:"agentEnvVars,omitempty" yaml:"agentEnvVars,omitempty"`
	AgentEnvVarsRKE1       *[]management.EnvVar                     `json:"agentEnvVarsRKE1,omitempty" yaml:"agentEnvVarsRKE1,omitempty"`
	ClusterAgent           *management.AgentDeploymentCustomization `json:"clusterAgent,omitempty" yaml:"clusterAgent,omitempty"`
	FleetAgent             *management.AgentDeploymentCustomization `json:"fleetAgent,omitempty" yaml:"fleetAgent,omitempty"`
	Etcd                   *rkev1.ETCD                              `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	LabelsAndAnnotations   *LabelsAndAnnotations                    `json:"labelsAndAnnotations,omitempty" yaml:"labelsAndAnnotations,omitempty"`
	Networking             *Networking                              `json:"networking,omitempty" yaml:"networking,omitempty"`
	Registries             *Registries                              `json:"registries,omitempty" yaml:"registries,omitempty"`
	UpgradeStrategy        *rkev1.ClusterUpgradeStrategy            `json:"upgradeStrategy,omitempty" yaml:"upgradeStrategy,omitempty"`
	Advanced               *Advanced                                `json:"advanced,omitempty" yaml:"advanced,omitempty"`
	ClusterSSHTests        []SSHTestCase                            `json:"clusterSSHTests,omitempty" yaml:"clusterSSHTests,omitempty"`
}
