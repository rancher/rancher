package componentchecks

import (
	"strings"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
)

// CheckETCDVersion will check the etcd version on the etcd node in the provisioned RKE1 cluster.
func CheckETCDVersion(client *rancher.Client, nodes []*nodes.Node, clusterID string) ([]string, error) {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	nodesList, err := steveClient.SteveType("node").List(nil)
	if err != nil {
		return nil, err
	}

	var etcdResult []string

	for _, rancherNode := range nodesList.Data {
		externalIP := rancherNode.Annotations["rke.cattle.io/external-ip"]
		etcdRole := rancherNode.Labels["node-role.kubernetes.io/etcd"] == "true"

		if etcdRole == true {
			for _, node := range nodes {
				if strings.Contains(node.PublicIPAddress, externalIP) {
					command := "docker exec etcd etcdctl version"
					output, err := node.ExecuteCommand(command)
					if err != nil {
						return []string{}, err
					}

					etcdResult = append(etcdResult, output)
					logrus.Infof(output)
				}
			}
		}
	}

	return etcdResult, nil
}
