package machinepool

import (
	"context"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/automation-framework/testclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	DOKind           = "DigitaloceanConfig"
	DOPoolType       = "rke-machine-config.cattle.io.digitaloceanconfig"
	DOResourceConfig = "digitaloceanconfigs"
	apiVersion       = "rke-machine-config.cattle.io/v1"
)

type MachineConfig struct {
	unstructured *unstructured.Unstructured
	client       *testclient.Client
}

func NewMachineConfig(generatedPoolName, kind, namespace, image, region, size string) *MachineConfig {
	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion(apiVersion)
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
	machineConfig.Object["type"] = DOPoolType
	machineConfig.Object["userdata"] = ""

	digitalOceanConfig := &MachineConfig{
		unstructured: machineConfig,
	}

	return digitalOceanConfig
}

// machineConfig := NewMachineConfig(...)
// machineConfig.Create(resource)

func (m *MachineConfig) Create(resource string) (*unstructured.Unstructured, error) {
	groupVersionResource := schema.GroupVersionResource{
		Group:    "rke-machine-config.cattle.io",
		Version:  "v1",
		Resource: resource,
	}

	ctx := context.Background()
	podResult, err := m.client.Dynamic.Resource(groupVersionResource).Namespace(m.unstructured.GetNamespace()).Create(ctx, m.unstructured, metav1.CreateOptions{})
	return podResult, err
}

func (m *MachineConfig) Cleanup() error {
	return nil
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
