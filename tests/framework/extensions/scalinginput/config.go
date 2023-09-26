package scalinginput

import (
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
)

const (
	ConfigurationFileKey = "scalingInput"
)

type Config struct {
	NodesAndRoles     *machinepools.NodeRoles `json:"nodesAndRoles" yaml:"nodesAndRoles"`
	NodesAndRolesRKE1 *nodepools.NodeRoles    `json:"nodesAndRolesRKE1" yaml:"nodesAndRolesRKE1"`
}
