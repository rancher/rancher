package nodes

import (
	"fmt"
	"net/url"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	active                   = "active"
	machineSteveResourceType = "cluster.x-k8s.io.machine"
	clusterLabel             = "cluster.x-k8s.io/cluster-name"
	PollInterval             = time.Duration(5 * time.Second)
	PollTimeout              = time.Duration(15 * time.Minute)
)

// IsNodeReady is a helper method that will loop and check if the node is ready in the RKE1 cluster.
// It will return an error if the node is not ready after set amount of time.
func IsNodeReady(client *rancher.Client, ClusterID string) error {
	err := wait.Poll(500*time.Millisecond, 30*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.ListAll(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": ClusterID,
			},
		})
		if err != nil {
			return false, err
		}

		for _, node := range nodes.Data {
			node, err := client.Management.Node.ByID(node.ID)
			if err != nil {
				return false, nil
			}

			if node.State != active {
				return false, nil
			}
		}
		logrus.Infof("All nodes in the cluster are in an active state!")
		return true, nil
	})

	return err
}

func IsRKE1EtcdNodeReplaced(client *rancher.Client, etcdNodeToDelete management.Node, clusterResp *management.Cluster, numOfEtcdNodesBeforeDeletion int) (bool, error) {
	numOfEtcdNodesAfterDeletion := 0

	err := wait.Poll(PollInterval, PollTimeout, func() (done bool, err error) {
		machines, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
			"clusterId": clusterResp.ID,
		}})
		if err != nil {
			return false, err
		}
		numOfEtcdNodesAfterDeletion = 0
		for _, machine := range machines.Data {
			if machine.Etcd {
				if machine.ID == etcdNodeToDelete.ID {
					return false, nil
				}
				numOfEtcdNodesAfterDeletion++
			}
		}
		logrus.Info("new etcd node : ")
		for _, machine := range machines.Data {
			if machine.Etcd {
				logrus.Info(machine.NodeName)
			}
		}
		return true, nil
	})
	return numOfEtcdNodesBeforeDeletion == numOfEtcdNodesAfterDeletion, err
}

func IsRKE2K3SNodeReplaced(client *rancher.Client, query url.Values, clusterName, nodeLabel string, nodeToDelete v1.SteveAPIObject, numNodesBeforeDeletion int) (bool, error) {
	numNodesAfterDeletion := 0
	err := wait.Poll(PollInterval, PollTimeout, func() (done bool, err error) {
		machines, err := client.Steve.SteveType(machineSteveResourceType).List(query)
		if err != nil {
			return false, err
		}

		numNodesAfterDeletion = 0
		for _, machine := range machines.Data {
			if machine.Labels[nodeLabel] == "true" && machine.Labels[clusterLabel] == clusterName {
				logrus.Info(fmt.Sprintf("%s: %s", nodeLabel, machine.Name))
				if machine.Name == nodeToDelete.Name {
					return false, nil
				}
				numNodesAfterDeletion++
			}
		}
		logrus.Info(fmt.Sprintf("%s node replaced: ", nodeLabel))
		for _, machine := range machines.Data {
			if machine.Labels[nodeLabel] == "true" && machine.Labels[clusterLabel] == clusterName {
				logrus.Info(machine.Name)
			}
		}
		return true, nil
	})
	return numNodesBeforeDeletion == numNodesAfterDeletion, err
}
