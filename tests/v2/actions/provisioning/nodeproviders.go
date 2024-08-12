package provisioning

import (
	"fmt"

	"github.com/rancher/rancher/tests/v2/actions/nodes/ec2"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/nodes"
)

const (
	ec2NodeProviderName = "ec2"
	fromConfig          = "config"
)

type NodeCreationFunc func(client *rancher.Client, rolesPerPool []string, quantityPerPool []int32) (nodes []*nodes.Node, err error)
type NodeDeletionFunc func(client *rancher.Client, nodes []*nodes.Node) error

type ExternalNodeProvider struct {
	Name             string
	NodeCreationFunc NodeCreationFunc
	NodeDeletionFunc NodeDeletionFunc
}

// ExternalNodeProviderSetup is a helper function that setups an ExternalNodeProvider object is a wrapper
// for the specific outside node provider node creator function
func ExternalNodeProviderSetup(providerType string) ExternalNodeProvider {
	switch providerType {
	case ec2NodeProviderName:
		return ExternalNodeProvider{
			Name:             providerType,
			NodeCreationFunc: ec2.CreateNodes,
			NodeDeletionFunc: ec2.DeleteNodes,
		}
	case fromConfig:
		return ExternalNodeProvider{
			Name: providerType,
			NodeCreationFunc: func(client *rancher.Client, rolesPerPool []string, quantityPerPool []int32) (nodesList []*nodes.Node, err error) {
				var nodeConfig nodes.ExternalNodeConfig
				config.LoadConfig(nodes.ExternalNodeConfigConfigurationFileKey, &nodeConfig)

				nodesList = nodeConfig.Nodes[-1]

				for _, node := range nodesList {
					sshKey, err := nodes.GetSSHKey(node.SSHKeyName)
					if err != nil {
						return nil, err
					}

					node.SSHKey = sshKey
				}
				return nodesList, nil
			},
			NodeDeletionFunc: func(client *rancher.Client, nodes []*nodes.Node) error {
				return ec2.DeleteNodes(client, nodes)
			},
		}
	default:
		panic(fmt.Sprintf("Node Provider:%v not found", providerType))
	}

}
