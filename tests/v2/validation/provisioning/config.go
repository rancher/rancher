package provisioning

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rke/types"
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

type AdvancedOptions struct {
	ClusterAgentCustomization management.AgentDeploymentCustomization `json:"clusterAgentCustomization" yaml:"clusterAgentCustomization"`
	FleetAgentCustomization   management.AgentDeploymentCustomization `json:"fleetAgentCustomization" yaml:"fleetAgentCustomization"`
}

type Config struct {
	NodesAndRoles            []machinepools.NodeRoles       `json:"nodesAndRoles" yaml:"nodesAndRoles" default:"[]"`
	NodesAndRolesRKE1        []nodepools.NodeRoles          `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1" default:"[]"`
	K3SKubernetesVersions    []string                       `json:"k3sKubernetesVersion" yaml:"k3sKubernetesVersion"`
	RKE1KubernetesVersions   []string                       `json:"rke1KubernetesVersion" yaml:"rke1KubernetesVersion"`
	RKE2KubernetesVersions   []string                       `json:"rke2KubernetesVersion" yaml:"rke2KubernetesVersion"`
	CNIs                     []string                       `json:"cni" yaml:"cni"`
	Providers                []string                       `json:"providers" yaml:"providers"`
	NodeProviders            []string                       `json:"nodeProviders" yaml:"nodeProviders"`
	PSACT                    string                         `json:"psact" yaml:"psact"`
	Hardened                 bool                           `json:"hardened" yaml:"hardened"`
	AdvancedOptions          AdvancedOptions                `json:"advancedOptions" yaml:"advancedOptions"`
	LocalClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint" yaml:"localClusterAuthEndpoint"`
	S3BackupConfig           *types.S3BackupConfig          `json:"s3BackupConfig" yaml:"s3BackupConfig"`
}
