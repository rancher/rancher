package provisioning

import (
<<<<<<< HEAD
	"strings"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/pkg/config"
=======
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
>>>>>>> refs/remotes/origin/add-rke2-ace-test
)

const (
	defaultRandStringLength = 5
	ConfigurationFileKey    = "provisioningInput"
)

type Config struct {
<<<<<<< HEAD
	NodesAndRoles            string                         `json:"nodesAndRoles" yaml:"nodesAndRoles" default:""`
	KubernetesVersions       []string                       `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNIs                     []string                       `json:"cni" yaml:"cni"`
	Providers                []string                       `json:"providers" yaml:"providers"`
	LocalClusterAuthEndpoint rkev1.LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint" yaml:"localClusterAuthEndpoint"`
=======
	NodesAndRoles      []machinepools.NodeRoles `json:"nodesAndRoles" yaml:"nodesAndRoles" default:"[]"`
	NodesAndRolesRKE1  []nodepools.NodeRoles    `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1" default:"[]"`
	KubernetesVersions []string                 `json:"kubernetesVersion" yaml:"kubernetesVersion"`
	CNIs               []string                 `json:"cni" yaml:"cni"`
	Providers          []string                 `json:"providers" yaml:"providers"`
	NodeProviders      []string                 `json:"nodeProviders" yaml:"nodeProviders"`
>>>>>>> refs/remotes/origin/add-rke2-ace-test
}

func AppendRandomString(baseClusterName string) string {
	clusterName := "auto-" + baseClusterName + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	return clusterName
}
