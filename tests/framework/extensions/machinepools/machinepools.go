package machinepools

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	ProvisioningSteveResourceType = "provisioning.cattle.io.cluster"
	active                        = "active"
)

func updateClusterMachinePool(client *rancher.Client, cluster *v1.SteveAPIObject, updatedCluster *apisV1.Cluster, newQuantity int32) (*v1.SteveAPIObject, error) {

	err := kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {
		clusterResp, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
		if clusterResp.ObjectMeta.State.Name == active {
			updateCluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
			if err != nil {
				return false, err
			}

			updatedCluster.ObjectMeta.ResourceVersion = updateCluster.ObjectMeta.ResourceVersion
			updatedCluster.Spec.RKEConfig.MachinePools[len(updatedCluster.Spec.RKEConfig.MachinePools)-1].Quantity = &newQuantity

			cluster, err = client.Steve.SteveType(ProvisioningSteveResourceType).Update(cluster, updatedCluster)
			if err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil
	})

	if err != nil {
		return nil, err
	}

	err = kwait.Poll(500*time.Millisecond, 10*time.Minute, func() (done bool, err error) {

		client, err = client.ReLogin()
		if err != nil {
			return false, err
		}

		clusterResp, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
		if err != nil {
			return false, err
		}

		if clusterResp.ObjectMeta.State.Name == active {
			return true, nil
		} else {
			return false, nil
		}
	})

	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// NewRKEMachinePool is a constructor that sets up a apisV1.RKEMachinePool object to be used to provision a cluster.
func NewRKEMachinePool(controlPlaneRole, etcdRole, workerRole bool, poolName string, quantity int32, machineConfig *v1.SteveAPIObject) apisV1.RKEMachinePool {
	machineConfigRef := &corev1.ObjectReference{
		Kind: machineConfig.Kind,
		Name: machineConfig.Name,
	}

	return apisV1.RKEMachinePool{
		ControlPlaneRole: controlPlaneRole,
		EtcdRole:         etcdRole,
		WorkerRole:       workerRole,
		NodeConfig:       machineConfigRef,
		Name:             poolName,
		Quantity:         &quantity,
	}
}

type NodeRoles struct {
	ControlPlane bool  `json:"controlplane,omitempty" yaml:"controlplane,omitempty"`
	Etcd         bool  `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Worker       bool  `json:"worker,omitempty" yaml:"worker,omitempty"`
	Windows      bool  `json:"windows,omitempty" yaml:"windows,omitempty"`
	Quantity     int32 `json:"quantity" yaml:"quantity"`
}

func (n NodeRoles) String() string {
	result := make([]string, 0, 3)
	if n.Quantity < 1 {
		return ""
	}
	if n.ControlPlane {
		result = append(result, "controlplane")
	}
	if n.Etcd {
		result = append(result, "etcd")
	}
	if n.Worker {
		result = append(result, "worker")
	}

	return fmt.Sprintf("%d %s", n.Quantity, strings.Join(result, "+"))
}

// MachinePoolSetup is a helper method that will loop and setup multiple node pools with the defined node roles from the `nodeRoles` parameter
// `machineConfig` is the *unstructured.Unstructured created by CreateMachineConfig
// `nodeRoles` would be in this format
//   []NodeRoles{
//   {
// 	   ControlPlane: true,
// 	   Etcd:         false,
// 	   Worker:       false,
//	   Quantity:     1,
//   },
//   {
// 	   ControlPlane: false,
// 	   Etcd:         true,
// 	   Worker:       false,
//	   Quantity:     1,
//   },
//  }

func RKEMachinePoolSetup(nodeRoles []NodeRoles, machineConfig *v1.SteveAPIObject) []apisV1.RKEMachinePool {
	machinePools := make([]apisV1.RKEMachinePool, 0, len(nodeRoles))
	for index, roles := range nodeRoles {
		machinePool := NewRKEMachinePool(roles.ControlPlane, roles.Etcd, roles.Worker, "pool"+strconv.Itoa(index), roles.Quantity, machineConfig)
		machinePools = append(machinePools, machinePool)
	}

	return machinePools
}

func CreateNewWorkerMachinePool(client *rancher.Client, cluster *v1.SteveAPIObject, updatedCluster *apisV1.Cluster, machineConfig *v1.SteveAPIObject, quantity int32) (error, *v1.SteveAPIObject, *apisV1.Cluster) {
	nodePoolConfig := []NodeRoles{
		{
			ControlPlane: false,
			Etcd:         false,
			Worker:       true,
			Quantity:     1,
		},
	}

	machinePools := RKEMachinePoolSetup(nodePoolConfig, machineConfig)
	machinePools[0].Name = "auto-pool-" + namegenerator.RandStringLower(5)
	updatedCluster.Spec.RKEConfig.MachinePools = append(updatedCluster.Spec.RKEConfig.MachinePools, machinePools...)

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err, nil, nil
	}

	logrus.Infof("Creating new worker machine pool...")

	updatedClusterResp, err := updateClusterMachinePool(adminClient, cluster, updatedCluster, quantity)
	if err != nil {
		return err, nil, nil
	}

	return nil, updatedClusterResp, updatedCluster
}

func ScaleNewWorkerMachinePool(client *rancher.Client, updatedClusterResp *v1.SteveAPIObject, updatedCluster *apisV1.Cluster, quantity int32) (error, *v1.SteveAPIObject) {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err, nil
	}

	logrus.Infof("Scaling machine pool to %v worker nodes...", quantity)
	scaledClusterResp, err := updateClusterMachinePool(adminClient, updatedClusterResp, updatedCluster, quantity)
	if err != nil {
		return err, nil
	}

	logrus.Infof("Machine pool is scaled to %v worker nodes!", quantity)

	return nil, scaledClusterResp
}

func DeleteWorkerMachinePool(client *rancher.Client, cluster *v1.SteveAPIObject, scaledClusterResp *v1.SteveAPIObject, updatedCluster *apisV1.Cluster) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}

	logrus.Infof("Deleting machine pool...")
	updateCluster, err := adminClient.Steve.SteveType(ProvisioningSteveResourceType).ByID(cluster.ID)
	if err != nil {
		return err
	}

	updatedCluster.ObjectMeta.ResourceVersion = updateCluster.ObjectMeta.ResourceVersion
	updatedCluster.Spec.RKEConfig.MachinePools = updatedCluster.Spec.RKEConfig.MachinePools[:len(updatedCluster.Spec.RKEConfig.MachinePools)-1]

	_, err = adminClient.Steve.SteveType(ProvisioningSteveResourceType).Update(scaledClusterResp, updatedCluster)
	if err != nil {
		return err
	}

	logrus.Infof("Machine pool deleted!")

	return nil
}
