package machinepool

import (
	"context"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/automation-framework/clients"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	DOKind           = "DigitaloceanConfig"
	DOPoolType       = "rke-machine-config.cattle.io.digitaloceanconfig"
	DOResourceConfig = "digitaloceanconfigs"
)

type DigitalOecanConfg struct {
	*unstructured.Unstructured
}

func NewDigitalOceanMachineConfig(generatedPoolName, namespace, image, region, size string) *DigitalOecanConfg {
	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(DOKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["accessToken"] = ""
	machineConfig.Object["image"] = image
	machineConfig.Object["backups"] = false
	machineConfig.Object["ipv6"] = false
	machineConfig.Object["monitoring"] = false
	machineConfig.Object["privateNetworking"] = false
	machineConfig.Object["region"] = region
	machineConfig.Object["size"] = size
	machineConfig.Object["sshKeyContents"] = ""
	machineConfig.Object["sshKeyFingerprint"] = ""
	machineConfig.Object["sshPort"] = "22"
	machineConfig.Object["sshUser"] = "root"
	machineConfig.Object["tags"] = ""
	machineConfig.Object["type"] = DOPoolType
	machineConfig.Object["userdata"] = ""

	digitalOceanConfig := &DigitalOecanConfg{
		machineConfig,
	}

	return digitalOceanConfig
}

func CreateMachineConfig(resource string, machinePoolConfig *unstructured.Unstructured, client *clients.Client) (*unstructured.Unstructured, error) {
	groupVersionResource := schema.GroupVersionResource{
		Group:    "rke-machine-config.cattle.io",
		Version:  "v1",
		Resource: resource,
	}

	ctx := context.Background()
	podResult, err := client.Dynamic.Resource(groupVersionResource).Namespace(machinePoolConfig.GetNamespace()).Create(ctx, machinePoolConfig, metav1.CreateOptions{})
	return podResult, err
}

func MachinePoolSetup(controlPlaneRole, etcdRole, workerRole bool, poolName string, quantity int32, machineConfig *unstructured.Unstructured) apisV1.RKEMachinePool {
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
