package machinepools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	provisioning "github.com/rancher/rancher/tests/framework/clients/rancher/generated/provisioning/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CreateMachineConfig is a helper method that creates the rke-machine-config, from any service provider available on rancher e.g. amazonec2configs
// This function uses the dynamic client create the rke-machine-config
func CreateMachineConfig(resource string, machinePoolConfig *unstructured.Unstructured, client *rancher.Client) (*unstructured.Unstructured, error) {
	groupVersionResource := schema.GroupVersionResource{
		Group:    "rke-machine-config.cattle.io",
		Version:  "v1",
		Resource: resource,
	}

	dynamic, err := client.GetRancherDynamicClient()
	if err != nil {
		return nil, err
	}

	return dynamic.Resource(groupVersionResource).Namespace(machinePoolConfig.GetNamespace()).Create(context.TODO(), machinePoolConfig, metav1.CreateOptions{})
}

// NewMachinePool is a constructor that sets up a apisV1.RKEMachinePool object to be used to provision a cluster.
func NewMachinePool(controlPlaneRole, etcdRole, workerRole bool, poolName string, quantity int64, machineConfig *unstructured.Unstructured) provisioning.RKEMachinePool {
	machineConfigRef := &provisioning.ObjectReference{
		Kind: machineConfig.GetKind(),
		Name: machineConfig.GetName(),
	}

	return provisioning.RKEMachinePool{
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
	Quantity     int64 `json:"quantity" yaml:"quantity"`
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

func MachinePoolSetup(nodeRoles []NodeRoles, machineConfig *unstructured.Unstructured) []provisioning.RKEMachinePool {
	machinePools := []provisioning.RKEMachinePool{}
	for index, roles := range nodeRoles {
		machinePool := NewMachinePool(roles.ControlPlane, roles.Etcd, roles.Worker, "pool"+strconv.Itoa(index), roles.Quantity, machineConfig)
		machinePools = append(machinePools, machinePool)
	}

	return machinePools
}
