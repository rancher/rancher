package provisioning

import (
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
)

type Version string

const (
	namespace                       = "fleet-default"
	defaultRandStringLength         = 5
	ConfigurationFileKey            = "provisioningInput"
	HardenedKubeVersion     Version = "v1.24.99"
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

type Config struct {
	NodesAndRoles          []machinepools.NodeRoles `json:"nodesAndRoles" yaml:"nodesAndRoles" default:"[]"`
	NodesAndRolesRKE1      []nodepools.NodeRoles    `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1" default:"[]"`
	K3SKubernetesVersions  []string                 `json:"k3sKubernetesVersion" yaml:"k3sKubernetesVersion"`
	RKE1KubernetesVersions []string                 `json:"rke1KubernetesVersion" yaml:"rke1KubernetesVersion"`
	RKE2KubernetesVersions []string                 `json:"rke2KubernetesVersion" yaml:"rke2KubernetesVersion"`
	CNIs                   []string                 `json:"cni" yaml:"cni"`
	Providers              []string                 `json:"providers" yaml:"providers"`
	NodeProviders          []string                 `json:"nodeProviders" yaml:"nodeProviders"`
	Hardened               bool                     `json:"hardened" yaml:"hardened"`
}
