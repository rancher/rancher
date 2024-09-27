package etcdsnapshot

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/rancher/norman/types"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/nodes"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	rancherv1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/etcdsnapshot"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sirupsen/logrus"
)

func MatchNodeToAnyEtcdRole(client *rancher.Client, clusterID string) (int, *management.Node) {
	machines, err := client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
		"clusterId": clusterID,
	}})
	if err != nil {
		return 0, nil
	}

	numOfNodes := 0
	lastMatchingNode := &management.Node{}

	for _, machine := range machines.Data {
		if machine.Etcd {
			lastMatchingNode = &machine
			numOfNodes++
		}
	}

	return numOfNodes, lastMatchingNode
}

// RKE1RetentionLimitCheck is a check that validates that the number of automatic snapshots on the cluster is under the retention limit
func RKE1RetentionLimitCheck(client *rancher.Client, clusterName string) error {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	clusterResp, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	retentionLimit := clusterResp.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.Retention
	s3Config := clusterResp.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.S3BackupConfig

	isS3 := false
	if s3Config != nil {
		isS3 = true
	}

	snapshotManagementObjList, err := client.Management.EtcdBackup.ListAll(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return err
	}

	automaticSnapshots := []management.EtcdBackup{}

	for _, snapshot := range snapshotManagementObjList.Data {
		if !snapshot.Manual {
			automaticSnapshots = append(automaticSnapshots, snapshot)
		}
	}

	listOpts := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/etcd=true"}
	etcdNodes, err := nodes.GetNodes(client, clusterID, listOpts)
	if err != nil {
		return err
	}

	expectedSnapshotsNum := int(retentionLimit) * len(etcdNodes)
	if isS3 {
		expectedSnapshotsNum = expectedSnapshotsNum * 2
	}

	if len(automaticSnapshots) > expectedSnapshotsNum {
		errMsg := fmt.Sprintf("retention limit exceeded: expected %d snapshots, found %d snapshots",
			expectedSnapshotsNum, len(automaticSnapshots))

		return errors.New(errMsg)
	}

	logrus.Infof("Snapshot retention limit respected, Snapshots Expected: %v Snapshots Found: %v",
		expectedSnapshotsNum, len(automaticSnapshots))

	return nil
}

// RKE2K3SRetentionLimitCheck is a check that validates that the number of automatic snapshots
// on the cluster is under the retention limit.
func RKE2K3SRetentionLimitCheck(client *rancher.Client, clusterName string) error {
	v1ClusterID, err := clusters.GetV1ProvisioningClusterByName(client, clusterName)
	if err != nil {
		return err
	}

	clusterObj, err := client.Steve.SteveType(stevetypes.Provisioning).ByID(v1ClusterID)
	if err != nil {
		return err
	}

	spec := apisV1.ClusterSpec{}
	err = rancherv1.ConvertToK8sType(clusterObj.Spec, &spec)
	if err != nil {
		return err
	}

	etcdConfig := spec.RKEConfig.ETCD
	retentionLimit := etcdConfig.SnapshotRetention

	isS3 := false
	if etcdConfig.S3 != nil {
		isS3 = true
	}

	query, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", etcdsnapshot.SnapshotClusterNameLabel, clusterName))
	if err != nil {
		return err
	}

	snapshotSteveObjList, err := client.Steve.SteveType(etcdsnapshot.SnapshotSteveResourceType).List(query)
	if err != nil {
		return err
	}

	automaticSnapshots := []rancherv1.SteveAPIObject{}

	for _, snapshot := range snapshotSteveObjList.Data {
		if strings.Contains(snapshot.Annotations["etcdsnapshot.rke.io/snapshot-file-name"], "etcd-snapshot") {
			automaticSnapshots = append(automaticSnapshots, snapshot)
		}
	}

	downstreamClusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	listOpts := metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/etcd=true"}
	etcdNodes, err := nodes.GetNodes(client, downstreamClusterID, listOpts)
	if err != nil {
		return err
	}

	expectedSnapshotsNum := int(retentionLimit) * len(etcdNodes)
	if isS3 {
		expectedSnapshotsNum = expectedSnapshotsNum * 2
	}

	if len(automaticSnapshots) > expectedSnapshotsNum {
		msg := fmt.Sprintf(
			"retention limit exceeded: expected %d snapshots, found %d snapshots",
			expectedSnapshotsNum, len(automaticSnapshots))

		return errors.New(msg)
	}

	logrus.Infof("Snapshot retention limit respected, Snapshots Expected: %v Snapshots Found: %v",
		expectedSnapshotsNum, len(automaticSnapshots))

	return nil
}
