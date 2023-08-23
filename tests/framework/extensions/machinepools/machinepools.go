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
func NewRKEMachinePool(controlPlaneRole, etcdRole, workerRole bool, poolName string, quantity int32, machineConfig *v1.SteveAPIObject, hostnameLengthLimit int) apisV1.RKEMachinePool {
	machineConfigRef := &corev1.ObjectReference{
		Kind: machineConfig.Kind,
		Name: machineConfig.Name,
	}
	machinePool := apisV1.RKEMachinePool{
		ControlPlaneRole: controlPlaneRole,
		EtcdRole:         etcdRole,
		WorkerRole:       workerRole,
		NodeConfig:       machineConfigRef,
		Name:             poolName,
		Quantity:         &quantity,
	}
	if hostnameLengthLimit > 0 {
		machinePool.HostnameLengthLimit = hostnameLengthLimit
	}
	return machinePool
}

type NodeRoles struct {
	ControlPlane bool  `json:"controlplane,omitempty" yaml:"controlplane,omitempty"`
	Etcd         bool  `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Worker       bool  `json:"worker,omitempty" yaml:"worker,omitempty"`
	Windows      bool  `json:"windows,omitempty" yaml:"windows,omitempty"`
	Quantity     int32 `json:"quantity" yaml:"quantity"`
}

// HostnameTruncation is a struct that is used to set the hostname length limit for a cluster or its pools during provisioning
type HostnameTruncation struct {
	PoolNameLengthLimit    int
	ClusterNameLengthLimit int
	Name                   string
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

// CreateAllMachinePools is a helper method that will loop and setup multiple node pools with the defined node roles from the `nodeRoles` parameter
// `machineConfig` is the *unstructured.Unstructured created by CreateMachineConfig
// `nodeRoles` would be in this format
//
//	  []NodeRoles{
//	  {
//		   ControlPlane: true,
//		   Etcd:         false,
//		   Worker:       false,
//		   Quantity:     1,
//	  },
//	  {
//		   ControlPlane: false,
//		   Etcd:         true,
//		   Worker:       false,
//		   Quantity:     1,
//	  },
//	 }
func CreateAllMachinePools(nodeRoles []NodeRoles, machineConfig *v1.SteveAPIObject, hostnameLengthLimits []HostnameTruncation) []apisV1.RKEMachinePool {
	machinePools := make([]apisV1.RKEMachinePool, 0, len(nodeRoles))
	hostnameLengthLimit := 0
	for index, roles := range nodeRoles {
		poolName := "pool" + strconv.Itoa(index)
		if hostnameLengthLimits != nil && len(hostnameLengthLimits) >= index {
			hostnameLengthLimit = hostnameLengthLimits[index].PoolNameLengthLimit
			poolName = hostnameLengthLimits[index].Name
		}
		if !roles.Windows {
			machinePool := NewRKEMachinePool(roles.ControlPlane, roles.Etcd, roles.Worker, poolName, roles.Quantity, machineConfig, hostnameLengthLimit)
			machinePools = append(machinePools, machinePool)
		} else {
			machinePool := NewRKEMachinePool(false, false, roles.Windows, poolName, roles.Quantity, machineConfig, hostnameLengthLimit)
			machinePools = append(machinePools, machinePool)
		}
	}

	return machinePools
}
