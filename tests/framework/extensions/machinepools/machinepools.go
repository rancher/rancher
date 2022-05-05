package machinepools

import (
	"context"
	"strconv"

	"github.com/rancher/rancher/pkg/controllers/provisioningv2/rke2"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CreateMachineConfig is a helper method that creates the rke-machine-config, from any service provider available on rancher ex) amazonec2configs
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

// NewRKEMachinePool is a constructor that sets up a apisV1.RKEMachinePool object to be used to provision a cluster.
func NewRKEMachinePool(controlPlaneRole, etcdRole, workerRole bool, poolName string, machineOS string, quantity int32, machineConfig *unstructured.Unstructured) apisV1.RKEMachinePool {
	machineConfigRef := &corev1.ObjectReference{
		Kind: machineConfig.GetKind(),
		Name: machineConfig.GetName(),
	}

	return apisV1.RKEMachinePool{
		ControlPlaneRole: controlPlaneRole,
		EtcdRole:         etcdRole,
		WorkerRole:       workerRole,
		NodeConfig:       machineConfigRef,
		Name:             poolName,
		Quantity:         &quantity,
		MachineOS:        machineOS,
	}
}

// RKEMachinePoolSetup is a helper method that will loop and setup muliple node pools with the defined node roles from the `nodeRoles` parameter
// `machineConfig` is the *unstructured.Unstructured created by CreateMachineConfig
// `nodeRoles` would be in this format
// []map[string]bool{
// {
// 	"controlplane": true,
// 	"etcd":         false,
// 	"worker":       false,
// },
// {
// 	"controlplane": false,
// 	"etcd":         true,
// 	"worker":       false,
// },
// }
func RKEMachinePoolSetup(nodeRoles []map[string]bool, hasWindows bool, machineConfig *unstructured.Unstructured) []apisV1.RKEMachinePool {
	var machinePools []apisV1.RKEMachinePool
	for index, roles := range nodeRoles {
		machinePool := NewRKEMachinePool(roles["controlplane"], roles["etcd"], roles["worker"], "pool"+strconv.Itoa(index), rke2.DefaultMachineOS, 1, machineConfig)
		if hasWindows {
			machinePool2019 := NewRKEMachinePool(false, false, true, "windows-pool-2019"+strconv.Itoa(index), rke2.WindowsMachineOS, 1, machineConfig)
			machinePool2022 := NewRKEMachinePool(false, false, true, "windows-pool-2022"+strconv.Itoa(index), rke2.WindowsMachineOS, 1, machineConfig)
			machinePools = append(machinePools, machinePool2019, machinePool2022)

		}
		machinePools = append(machinePools, machinePool)
	}

	return machinePools
}
