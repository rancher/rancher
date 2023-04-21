package machinepools

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	VmwaresphereKind                               = "VmwarevsphereConfig"
	VmwarevsphereType                              = "rke-machine-config.cattle.io.vmwarevsphereconfig"
	VmwarevsphereConfig                            = "vmwarevsphereconfigs"
	VmwarevsphereMachineConfigConfigurationFileKey = "vmwarevsphereMachineConfig"
)

// VsphereMachineConfig is configuration needed to create an rke-machine-config.cattle.io.vmwarevsphereconfig
type VmwarevsphereMachineConfig struct {
	Cfgparam              []string `json:"cfgparam" yaml:"cfgparam"`
	CloneFrom             string   `json:"cloneFrom" yaml:"cloneFrom"`
	CloudConfig           string   `json:"cloudConfig" yaml:"cloudConfig"`
	Cloundinit            string   `json:"cloundinit" yaml:"cloundinit"`
	ContentLibrary        string   `json:"contentLibrary" yaml:"contentLibrary"`
	CPUCount              string   `json:"cpuCount" yaml:"cpuCount"`
	CreationType          string   `json:"creationType" yaml:"creationType"`
	CustomAttribute       []string `json:"customAttribute" yaml:"customAttribute"`
	DataCenter            string   `json:"dataCenter" yaml:"dataCenter"`
	DataStore             string   `json:"dataStore" yaml:"dataStore"`
	DatastoreCluster      string   `json:"datastoreCluster" yaml:"datastoreCluster"`
	DiskSize              string   `json:"diskSize" yaml:"diskSize"`
	Folder                string   `json:"folder" yaml:"folder"`
	HostSystem            string   `json:"hostSystem" yaml:"hostSystem"`
	MemorySize            string   `json:"memorySize" yaml:"memorySize"`
	Network               []string `json:"network" yaml:"network"`
	OS                    string   `json:"os" yaml:"os"`
	Password              string   `json:"password" yaml:"password"`
	Pool                  string   `json:"pool" yaml:"pool"`
	SSHPassword           string   `json:"sshPassword" yaml:"sshPassword"`
	SSHPort               string   `json:"sshPort" yaml:"sshPort"`
	SSHUser               string   `json:"sshUser" yaml:"sshUser"`
	SSHUserGroup          string   `json:"sshUserGroup" yaml:"sshUserGroup"`
	Tag                   []string `json:"tag" yaml:"tag"`
	Username              string   `json:"username" yaml:"username"`
	VappIpallocationplicy string   `json:"vappIpallocationplicy" yaml:"vappIpallocationplicy"`
	VappIpprotocol        string   `json:"vappIpprotocol" yaml:"vappIpprotocol"`
	VappProperty          []string `json:"vappProperty" yaml:"vappProperty"`
	VappTransport         string   `json:"vappTransport" yaml:"vappTransport"`
	Vcenter               string   `json:"vcenter" yaml:"vcenter"`
	VcenterPort           string   `json:"vcenterPort" yaml:"vcenterPort"`
}

// NewVSphereMachineConfig is a constructor to set up rke-machine-config.cattle.io.vmwarevsphereconfig. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewVSphereMachineConfig(generatedPoolName, namespace string) *unstructured.Unstructured {
	var vmwarevsphereMachineConfig VmwarevsphereMachineConfig
	config.LoadConfig(VmwarevsphereMachineConfigConfigurationFileKey, &vmwarevsphereMachineConfig)

	machineConfig := &unstructured.Unstructured{}
	machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
	machineConfig.SetKind(AWSKind)
	machineConfig.SetGenerateName(generatedPoolName)
	machineConfig.SetNamespace(namespace)
	machineConfig.Object["cfgParam"] = vmwarevsphereMachineConfig.Cfgparam
	machineConfig.Object["cloneFrom"] = vmwarevsphereMachineConfig.CloneFrom
	machineConfig.Object["cloudConfig"] = vmwarevsphereMachineConfig.CloudConfig
	machineConfig.Object["cloundInit"] = vmwarevsphereMachineConfig.Cloundinit
	machineConfig.Object["contentLibrary"] = vmwarevsphereMachineConfig.ContentLibrary
	machineConfig.Object["cpuCount"] = vmwarevsphereMachineConfig.CPUCount
	machineConfig.Object["creationType"] = vmwarevsphereMachineConfig.CreationType
	machineConfig.Object["customAttribute"] = vmwarevsphereMachineConfig.CustomAttribute
	machineConfig.Object["dataCenter"] = vmwarevsphereMachineConfig.DataCenter
	machineConfig.Object["dataStore"] = vmwarevsphereMachineConfig.DataStore
	machineConfig.Object["datastoreCluster"] = vmwarevsphereMachineConfig.DatastoreCluster
	machineConfig.Object["diskSize"] = vmwarevsphereMachineConfig.DiskSize
	machineConfig.Object["folder"] = vmwarevsphereMachineConfig.Folder
	machineConfig.Object["hostSystem"] = vmwarevsphereMachineConfig.HostSystem
	machineConfig.Object["memorySize"] = vmwarevsphereMachineConfig.MemorySize
	machineConfig.Object["network"] = vmwarevsphereMachineConfig.Network
	machineConfig.Object["os"] = vmwarevsphereMachineConfig.OS
	machineConfig.Object["password"] = vmwarevsphereMachineConfig.Password
	machineConfig.Object["pool"] = vmwarevsphereMachineConfig.Pool
	machineConfig.Object["sshPassword"] = vmwarevsphereMachineConfig.SSHPassword
	machineConfig.Object["sshPort"] = vmwarevsphereMachineConfig.SSHPort
	machineConfig.Object["sshUser"] = vmwarevsphereMachineConfig.SSHUser
	machineConfig.Object["sshUserGroup"] = vmwarevsphereMachineConfig.SSHUserGroup
	machineConfig.Object["tag"] = vmwarevsphereMachineConfig.Tag
	machineConfig.Object["username"] = vmwarevsphereMachineConfig.Username
	machineConfig.Object["vappIpallocationplicy"] = vmwarevsphereMachineConfig.VappIpallocationplicy
	machineConfig.Object["vappIpprotocol"] = vmwarevsphereMachineConfig.VappIpprotocol
	machineConfig.Object["vappProperty"] = vmwarevsphereMachineConfig.VappProperty
	machineConfig.Object["vappTransport"] = vmwarevsphereMachineConfig.VappTransport
	machineConfig.Object["vcenter"] = vmwarevsphereMachineConfig.Vcenter
	machineConfig.Object["vcenterPort"] = vmwarevsphereMachineConfig.VcenterPort

	return machineConfig
}
