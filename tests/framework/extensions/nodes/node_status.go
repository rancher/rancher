package nodes

import (
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	activeState              = "active"
	runningState             = "running"
	errorState               = "error"
	machineSteveResourceType = "cluster.x-k8s.io.machine"
	machineSteveAnnotation   = "cluster.x-k8s.io/machine"
	fleetNamespace           = "fleet-default"
	etcdLabel                = "rke.cattle.io/etcd-role"
	clusterLabel             = "cluster.x-k8s.io/cluster-name"
	PollInterval             = time.Duration(5 * time.Second)
	PollTimeout              = time.Duration(15 * time.Minute)
)

// AllManagementNodeReady is a helper method that will loop and check if the node is ready in the RKE1 cluster.
// It will return an error if the node is not ready after set amount of time.
func AllManagementNodeReady(client *rancher.Client, ClusterID string) error {
	err := wait.Poll(1*time.Second, 30*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.ListAll(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": ClusterID,
			},
		})
		if err != nil {
			return false, nil
		}

		for _, node := range nodes.Data {
			node, err := client.Management.Node.ByID(node.ID)
			if err != nil {
				return false, nil
			}
			if node.State == errorState {
				logrus.Warnf("node %s is in error state", node.Name)
				return false, nil
			}
			if node.State != activeState {
				return false, nil
			}
		}
		logrus.Infof("All nodes in the cluster are in an active state!")
		return true, nil
	})

	return err
}

// AllMachineReady is a helper method that will loop and check if the machine object of every node in a cluster is ready. Typically Used for RKE2/K3s Clusters.
// It will return an error if the machine object is not ready after set amount of time.
func AllMachineReady(client *rancher.Client, clusterID string) error {
	err := wait.Poll(1*time.Second, 30*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
			"clusterId": clusterID,
		}})
		if err != nil {
			return false, err
		}
		for _, node := range nodes.Data {
			machine, err := client.Steve.SteveType(machineSteveResourceType).ByID(fleetNamespace + "/" + node.Annotations[machineSteveAnnotation])
			if err != nil {
				return false, err
			}
			if machine.State == nil {
				logrus.Infof("Machine: %s state is nil", machine.Name)
				return false, nil
			}
			if machine.State.Error {
				logrus.Warnf("Machine: %s is in error state: %s", machine.Name, machine.State.Message)
				return false, nil
			}
			if machine.State.Name != runningState {
				return false, nil
			}
		}
		logrus.Infof("All nodes in the cluster are running!")
		return true, nil
	})
	return err
}

// AllNodeDeleted is a helper method that will loop and check if the node is deleted in the cluster.
func AllNodeDeleted(client *rancher.Client, ClusterID string) error {
	err := wait.Poll(500*time.Millisecond, 5*time.Minute, func() (bool, error) {
		nodes, err := client.Management.Node.ListAll(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": ClusterID,
			},
		})
		if err != nil {
			return false, err
		}

		if len(nodes.Data) == 0 {
			logrus.Infof("All nodes in the cluster are deleted!")
			return true, nil
		}

		return false, nil
	})

	return err
}

// IsNodeReplaced is a helper method that will loop and check if the node matching its type is replaced in a cluster.
// It will return an error if the node is not replaced after set amount of time.
func IsNodeReplaced(client *rancher.Client, oldMachineID string, clusterID string, numOfNodesBeforeDeletion int, isEtcd bool, isControlPlane bool, isWorker bool) (bool, error) {
	numOfNodesAfterDeletion := 0

	err := wait.Poll(PollInterval, PollTimeout, func() (done bool, err error) {
		machines, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
			"clusterId": clusterID,
		}})
		if err != nil {
			return false, err
		}
		numOfNodesAfterDeletion = 0
		for _, machine := range machines.Data {
			if machine.Etcd == isEtcd && machine.ControlPlane == isControlPlane && machine.Worker == isWorker {
				if machine.ID == oldMachineID {
					return false, nil
				}
				logrus.Info("new node : ", machine.NodeName)
				numOfNodesAfterDeletion++
			}
		}
		return true, nil
	})
	return numOfNodesBeforeDeletion == numOfNodesAfterDeletion, err
}
