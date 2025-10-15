package autoscaler

import (
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/generated/controllers/cluster.x-k8s.io/v1beta1"
	v1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

type machineDeploymentReplicaOverrider struct {
	clusterCache  v1.ClusterCache
	clusterClient v1.ClusterClient

	capiClusterCache v1beta1.ClusterCache
}

// syncMachinePoolReplicas synchronizes machine pool replicas between the capi MachineDeployment and v2prov Cluster object's machinePool field.
// it searches through the list of machinePools and finds the matching one which corresponds to the one the cluster-autoscaler updated, and then updates the quantity field. this triggers a scale up (or scale down).
func (s *machineDeploymentReplicaOverrider) syncMachinePoolReplicas(_ string, md *capi.MachineDeployment) (*capi.MachineDeployment, error) {
	if md == nil || md.DeletionTimestamp != nil {
		return md, nil
	}

	clusterName := md.Spec.Template.ObjectMeta.Labels[capi.ClusterNameLabel]
	if clusterName == "" {
		logrus.Debugf("MachineDeployment %s/%s has no cluster name label, skipping", md.Namespace, md.Name)
		return md, nil
	}

	machinePoolName := md.Spec.Template.ObjectMeta.Labels[capr.RKEMachinePoolNameLabel]
	if machinePoolName == "" {
		logrus.Debugf("MachineDeployment %s/%s has no machine pool name label, skipping", md.Namespace, md.Name)
		return md, nil
	}

	logrus.Debugf("Getting CAPI Cluster %s/%s", md.Namespace, clusterName)
	capiCluster, err := s.capiClusterCache.Get(md.Namespace, clusterName)
	if err != nil {
		logrus.Errorf("Error getting capi cluster %s/%s: %v", md.Namespace, clusterName, err)
		return nil, err
	}

	logrus.Debugf("Getting v2prov cluster capi cluster %s/%s", capiCluster.Namespace, capiCluster.Name)
	cluster, err := capr.GetProvisioningClusterFromCAPICluster(capiCluster, s.clusterCache)
	if err != nil {
		return nil, err
	}

	if cluster.Spec.RKEConfig == nil || cluster.Spec.RKEConfig.MachinePools == nil || len(cluster.Spec.RKEConfig.MachinePools) == 0 {
		return md, nil
	}

	needUpdate := false
	cluster = cluster.DeepCopy()
	for i := range cluster.Spec.RKEConfig.MachinePools {
		if !(cluster.Spec.RKEConfig.MachinePools[i].Name == machinePoolName) {
			continue
		}

		if cluster.Spec.RKEConfig.MachinePools[i].Quantity == nil || md.Spec.Replicas == nil {
			continue
		}

		logrus.Debugf("Found matching machine pool %s", machinePoolName)
		if *cluster.Spec.RKEConfig.MachinePools[i].Quantity != *md.Spec.Replicas {
			logrus.Infof("Updating cluster %s/%s machine pool %s quantity from %d to %d",
				cluster.Namespace, cluster.Name, machinePoolName,
				*cluster.Spec.RKEConfig.MachinePools[i].Quantity, *md.Spec.Replicas)
			cluster.Spec.RKEConfig.MachinePools[i].Quantity = md.Spec.Replicas
			needUpdate = true
		}
	}

	if needUpdate {
		logrus.Debugf("Updating cluster %s/%s", cluster.Namespace, cluster.Name)
		err := wait.ExponentialBackoff(retry.DefaultBackoff, func() (done bool, err error) {
			_, err = s.clusterClient.Update(cluster)
			if err != nil {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			logrus.Warnf("Failed to update cluster %s/%s machine pool %s to match machineDeployment: %v",
				cluster.Namespace, cluster.Name, machinePoolName, err)
			return nil, err
		}

		logrus.Debugf("Successfully updated cluster %s/%s", cluster.Namespace, cluster.Name)
	}

	return md, nil
}
