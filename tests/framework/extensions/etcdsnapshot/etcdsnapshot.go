package etcdsnapshot

import (
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
)

const (
	ProvisioningSteveResouceType = "provisioning.cattle.io.cluster"
	fleetNamespace               = "fleet-default"
)

// CreateSnapshot is a helper function to create a snapshot on an RKE2 or k3s cluster. Returns error if any.
func CreateSnapshot(client *rancher.Client, clusterName string) error {
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

	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(clusterSteveObject, clusterObject)
	if err != nil {
		return err
	}

	return nil
}

func RestoreSnapshot(client *rancher.Client, clusterName string, snapshotRestore *rkev1.ETCDSnapshotRestore) error {
	clusterObj, existingSteveAPIObj, err := clusters.GetProvisioningClusterByName(client, clusterName, fleetNamespace)
	if err != nil {
		return err
	}

	clusterObj.Spec.RKEConfig.ETCDSnapshotRestore = snapshotRestore

	_, err = client.Steve.SteveType(ProvisioningSteveResouceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}
