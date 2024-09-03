package node

import (
	"time"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/nodes/ec2"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/defaults/stevestates"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/sshkeys"
	"github.com/rancher/shepherd/extensions/steve"
	"github.com/rancher/shepherd/pkg/nodes"
	"github.com/sirupsen/logrus"
)

const (
	provider = "provider.cattle.io"
)

type SSHCluster struct {
	id    string
	nodes []*nodes.Node
}

// NodeRebootTest reboots all nodes in the provided clusters, one per cluster at a time.
func NodeRebootTest(client *rancher.Client, clusterIDs []string) error {
	var clusters []*v1.SteveAPIObject
	var err error

	for _, clusterID := range clusterIDs {
		cluster, err := client.Steve.SteveType(stevetypes.Provisioning).ByID(clusterID)
		if err != nil {
			return err
		}

		clusters = append(clusters, cluster)
	}

	var SSHClusters []SSHCluster
	maxNodeNum := 0
	for _, cluster := range clusters {
		var err error
		var sshUser string
		if cluster.Labels[provider] == "rke" {
			logrus.Info("1")
			clusterStatus := &provv1.ClusterStatus{}
			err := v1.ConvertToK8sType(cluster.Status, clusterStatus)
			if err != nil {
				return err
			}
			logrus.Info("2")

			mgmtCluster, err := client.Management.Cluster.ByID(clusterStatus.ClusterName)
			if err != nil {
				return err
			}

			sshUser = mgmtCluster.AppliedSpec.RancherKubernetesEngineConfig.Nodes[0].User
		} else {
			sshUser, err = sshkeys.GetSSHUser(client, cluster)
		}

		if err != nil {
			return err
		}

		steveClient, err := steve.GetClusterClient(client, cluster.ID)
		if err != nil {
			return err
		}

		nodesSteveObjList, err := steveClient.SteveType(stevetypes.Node).List(nil)
		if err != nil {
			return err
		}
		logrus.Info("3")
		logrus.Info(len(nodesSteveObjList.Data))
		var sshNodes []*nodes.Node
		for _, node := range nodesSteveObjList.Data {
			logrus.Info(node.Annotations["cluster.x-k8s.io/machine"])
			clusterNode, err := sshkeys.GetSSHNodeFromMachine(client, sshUser, &node)
			if err != nil {
				return err
			}

			sshNodes = append(sshNodes, clusterNode)
		}

		if len(sshNodes) > maxNodeNum {
			maxNodeNum = len(sshNodes)
		}

		SSHClusters = append(SSHClusters, SSHCluster{id: cluster.ID, nodes: sshNodes})
	}
	logrus.Info("4")
	for i := range maxNodeNum {
		for _, cluster := range SSHClusters {
			if i > len(cluster.nodes) {
				continue
			}

			err := ec2.RebootNode(client, *cluster.nodes[i], cluster.id)
			if err != nil {
				return err
			}
		}

		for _, cluster := range clusters {
			err := steve.WaitForResourceState(client.Steve, cluster, stevestates.Active, time.Second, defaults.FifteenMinuteTimeout)
			if err != nil {
				return err
			}
		}
	}

	return err
}
