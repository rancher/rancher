package etcdsnapshot

import (
	"strings"

	"github.com/rancher/norman/types"
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/sirupsen/logrus"
)

const (
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	fleetNamespace               = "fleet-default"
	active                       = "active"
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

// GetRKE1Snapshots is a helper function to get the existing snapshots for a downstream RKE1 cluster.
func GetRKE1Snapshots(client *rancher.Client, clusterName string) ([]string, error) {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return nil, err
	}

	snapshotSteveObjList, err := client.Management.EtcdBackup.ListAll(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return nil, err
	}

	snapshots := []string{}
	for _, snapshot := range snapshotSteveObjList.Data {
		if strings.Contains(snapshot.Name, clusterID) {
			snapshots = append(snapshots, snapshot.ID)
		}
	}

	return snapshots, nil
}

// GetRKE2K3SSnapshots is a helper function to get the existing snapshots for a downstream RKE2/K3S cluster.
func GetRKE2K3SSnapshots(client *rancher.Client, localclusterID string, clusterName string) ([]string, error) {
	steveclient, err := client.Steve.ProxyDownstream(localclusterID)
	if err != nil {
		return nil, err
	}

	snapshotSteveObjList, err := steveclient.SteveType("rke.cattle.io.etcdsnapshot").List(nil)
	if err != nil {
		return nil, err
	}

	snapshots := []string{}
	for _, snapshot := range snapshotSteveObjList.Data {
		if strings.Contains(snapshot.ObjectMeta.Name, clusterName) {
			snapshots = append(snapshots, snapshot.Name)
		}
	}

	return snapshots, nil
}

// CreateRKE1Snapshot is a helper function to create a snapshot on an RKE1 cluster. Returns error if any.
func CreateRKE1Snapshot(client *rancher.Client, clusterName string) error {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	clusterResp, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	backupConfig := clusterResp.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig

	etcdBackup := &management.EtcdBackup{
		BackupConfig: &management.BackupConfig{
			Enabled:        backupConfig.Enabled,
			IntervalHours:  backupConfig.IntervalHours,
			Retention:      backupConfig.Retention,
			S3BackupConfig: backupConfig.S3BackupConfig,
			SafeTimestamp:  backupConfig.SafeTimestamp,
			Timeout:        backupConfig.Timeout,
		},
		ClusterID: clusterID,
		Manual:    false,
		Name:      clusterID + "-" + namegen.AppendRandomString(""),
		Status: &management.EtcdBackupStatus{
			KubernetesVersion: clusterResp.RancherKubernetesEngineConfig.Version,
		},
	}

	logrus.Infof("Creating snapshot...")
	_, err = client.Management.EtcdBackup.Create(etcdBackup)
	if err != nil {
		return err
	}

	return nil
}

// CreateRKE2K3SSnapshot is a helper function to create a snapshot on an RKE2 or k3s cluster. Returns error if any.
func CreateRKE2K3SSnapshot(client *rancher.Client, clusterName string) error {
	clusterObject, clusterSteveObject, err := clusters.GetProvisioningClusterByName(client, clusterName, fleetNamespace)
	if err != nil {
		return err
	}

	if clusterObject.Spec.RKEConfig != nil {
		if clusterObject.Spec.RKEConfig.ETCDSnapshotCreate == nil {
			clusterObject.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
				Generation: 1,
			}
		} else {
			clusterObject.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
				Generation: clusterObject.Spec.RKEConfig.ETCDSnapshotCreate.Generation + 1,
			}
		}
	} else {
		clusterObject.Spec.RKEConfig = &apisV1.RKEConfig{
			ETCDSnapshotCreate: &rkev1.ETCDSnapshotCreate{
				Generation: 1,
			},
		}
	}

	logrus.Infof("Creating snapshot...")
	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(clusterSteveObject, clusterObject)
	if err != nil {
		return err
	}

	return nil
}

// RestoreRKE1Snapshot is a helper function to restore a snapshot on an RKE1 cluster. Returns error if any.
func RestoreRKE1Snapshot(client *rancher.Client, clusterName string, snapshotRestore *management.RestoreFromEtcdBackupInput, kubernetesVersion string) error {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	clusterResp, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	clusterResp.RancherKubernetesEngineConfig.Restore.Restore = true
	clusterResp.RancherKubernetesEngineConfig.Restore.SnapshotName = snapshotRestore.EtcdBackupID
	clusterResp.RancherKubernetesEngineConfig.Version = kubernetesVersion

	logrus.Infof("Restoring snapshot: %v", snapshotRestore.EtcdBackupID)
	_, err = client.Management.Cluster.Update(clusterResp, &clusterResp)
	if err != nil {
		return err
	}

	return nil
}

// CreateRKE2K3SSnapshot is a helper function to restore a snapshot on an RKE2 or k3s cluster. Returns error if any.
func RestoreRKE2K3SSnapshot(client *rancher.Client, clusterName string, snapshotRestore *rkev1.ETCDSnapshotRestore) error {
	clusterObj, existingSteveAPIObj, err := clusters.GetProvisioningClusterByName(client, clusterName, fleetNamespace)
	if err != nil {
		return err
	}

	clusterObj.Spec.RKEConfig.ETCDSnapshotRestore = snapshotRestore

	logrus.Infof("Restoring snapshot: %v", snapshotRestore.Name)
	_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}
