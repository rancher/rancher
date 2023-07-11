package machinepools

import (
	"fmt"
	"strconv"
	"strings"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	corev1 "k8s.io/api/core/v1"
)

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
