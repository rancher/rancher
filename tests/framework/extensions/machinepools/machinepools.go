package machinepools

import (
	"context"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CreateMachineConfig is a helper method that actually creates the rke-machine-config
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

func NewRKEMachinePool(controlPlaneRole, etcdRole, workerRole bool, poolName string, quantity int32, machineConfig *unstructured.Unstructured) apisV1.RKEMachinePool {
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
	}
}
