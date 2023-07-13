package etcdsnapshot

import (
	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
)

const (
	ProvisioningSteveResourceType = "provisioning.cattle.io.cluster"
)

// CreateSnapshot is a helper function to create a snapshot on an RKE2 or k3s cluster. Returns error if any.
func CreateSnapshot(client *rancher.Client, clustername string, namespace string) error {
	clusterObj, existingSteveAPIObj, err := clusters.GetProvisioningClusterByName(client, clustername, namespace)
	if err != nil {
		return err
	}

	clusterSpec := &apisV1.ClusterSpec{}
	err = steveV1.ConvertToK8sType(clusterObj.Spec, clusterSpec)
	if err != nil {
		return err
	}

	if clusterSpec.RKEConfig.ETCDSnapshotCreate != nil {
		clusterObj.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
			Generation: clusterObj.Spec.RKEConfig.ETCDSnapshotCreate.Generation + 1,
		}
	} else {
		clusterObj.Spec.RKEConfig.ETCDSnapshotCreate = &rkev1.ETCDSnapshotCreate{
			Generation: 1,
		}
	}

	_, err = client.Steve.SteveType(clusters.ProvisioningSteveResourceType).Update(existingSteveAPIObj, clusterObj)
	if err != nil {
		return err
	}

	return nil
}
