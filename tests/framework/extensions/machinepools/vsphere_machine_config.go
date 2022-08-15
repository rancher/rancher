package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	VSphereKind                                    = "VmwarevsphereConfig"
	VSpherePoolType                                = "rke-machine-config.cattle.io.vmwarevsphereconfig"
	VSphereResourceConfig                          = "vmwarevsphereconfigs"
	VmwareVsphereMachingConfigConfigurationFileKey = "vmwarevsphereMachineConfig"
)

type VSpherePublicCredentials struct {
	Username    string `json:"username" yaml:"username"`
	VCenter     string `json:"vcenter" yaml:"vcenter"`
	VCenterPort string `json:"vcenterPort" yaml:"vcenterPort"`
}

type VSpherePrivateCredentials struct {
	// TODO: use sha256sum
	Password string `json:"password" yaml:"password"`
}

type VSphereMachineConfig struct {
	DiskSize         string   `json:"diskSize" yaml:"diskSize"`
	CPUCount         string   `json:"cpuCount" yaml:"cpuCount"`
	MemorySize       string   `json:"memorySize" yaml:"memorySize"`
	NetworkName      string   `json:"networkName" yaml:"networkName"`
	CreationType     string   `json:"creationType" yaml:"creationType"`
	SSHUser          string   `json:"sshUser" yaml:"sshUser"`
	SSHGroup         string   `json:"sshGroup" yaml:"sshGroup"`
	CloneFrom        string   `json:"cloneFrom" yaml:"cloneFrom"`
	ResourcePool     string   `json:"resourcePool" yaml:"resourcePool"`
	Datastore        string   `json:"datastore" yaml:"datastore"`
	DatastoreCluster string   `json:"datastoreCluster" yaml:"datastoreCluster"`
	VMFolder         string   `json:"vmFolder" yaml:"VMFolder"`
	VSphereTags      []string `json:"vSphereTag" yaml:"vSphereTag"`
	Template         string   `json:"template" yaml:"template"`
	CloudConfig      []byte   `json:"cloudConfig" yaml:"cloudConfig"`
}

type VSphereNodeTemplate struct {
	VSphereMachineConfig
	CloudConfig       []byte   `json:"cloudConfig" yaml:"cloudConfig"`
	Namespace         string   `json:"namespace" yaml:"namespace"`
	CloudCredentialID string   `json:"cloudCredentialID" yaml:"cloudCredentialID"`
	VMFolder          string   `json:"vmFolder" yaml:"VMFolder"`
	VSphereTags       []string `json:"vSphereTag" yaml:"vSphereTag"`
	Driver            string   `json:"driver" yaml:"driver"`
	Datastore         string   `json:"datastore" yaml:"datastore"`
	DatastoreCluster  string   `json:"datastoreCluster" yaml:"datastoreCluster"`
}

// NewVsphereMachineConfig is a constructor to set up rke-machine-config.cattle.io.vmwarevsphereconfig.
// It returns an *unstructured.Unstructured that CreateMachineConfig uses to create the rke-machine-config
func NewVsphereMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var vsphereMachineConfig VSphereMachineConfig
	config.LoadConfig(VmwareVsphereMachingConfigConfigurationFileKey, &vsphereMachineConfig)
	machineConfig := &unstructured.Unstructured{}

	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(VSphereKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)

	machineConfig.Object["diskSize"] = vsphereMachineConfig.DiskSize
	machineConfig.Object["cpuCount"] = vsphereMachineConfig.CPUCount
	machineConfig.Object["memorySize"] = vsphereMachineConfig.MemorySize
	machineConfig.Object["networkName"] = vsphereMachineConfig.NetworkName
	machineConfig.Object["cloudConfig"] = vsphereMachineConfig.CloudConfig
	machineConfig.Object["datastore"] = vsphereMachineConfig.Datastore
	machineConfig.Object["datastoreCluster"] = vsphereMachineConfig.DatastoreCluster
	machineConfig.Object["sshUser"] = vsphereMachineConfig.SSHUser
	machineConfig.Object["sshGroup"] = vsphereMachineConfig.SSHGroup
	machineConfig.Object["creationType"] = vsphereMachineConfig.CreationType
	machineConfig.Object["cloneFrom"] = vsphereMachineConfig.CloneFrom
	machineConfig.Object["resourcePool"] = vsphereMachineConfig.ResourcePool
	machineConfig.Object["vMFolder"] = vsphereMachineConfig.VMFolder
	machineConfig.Object["sshGroup"] = vsphereMachineConfig.CreationType

	return machineConfig
}
