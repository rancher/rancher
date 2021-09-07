package machinepool

import (
	"context"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

const (
	DOKind     = "DigitaloceanConfig"
	DOPoolType = "rke-machine-config.cattle.io.digitaloceanconfig"
)

func NewMachinePoolConfig(generatedPoolName, kind, namespace, poolType, image, region, size string) *unstructured.Unstructured {
	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(kind)
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
	machineConfig.Object["type"] = poolType
	machineConfig.Object["userdata"] = ""
	return machineConfig
}

func CreateMachineConfigPool(machinePoolConfig *unstructured.Unstructured, client dynamic.NamespaceableResourceInterface) (*unstructured.Unstructured, error) {
	ctx := context.Background()
	podResult, err := client.Namespace(machinePoolConfig.GetNamespace()).Create(ctx, machinePoolConfig, metav1.CreateOptions{})
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
