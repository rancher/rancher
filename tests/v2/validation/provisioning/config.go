package provisioning

import (
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
)

const (
	ConfigurationFileKey = "provisioningInput"
)

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
