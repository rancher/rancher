package machinepools

import (
	"github.com/rancher/shepherd/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	VmwaresphereKind                               = "VmwarevsphereConfig"
	VmwarevsphereType                              = "rke-machine-config.cattle.io.vmwarevsphereconfig"
	VmwarevsphereConfig                            = "vmwarevsphereconfigs"
	VmwarevsphereMachineConfigConfigurationFileKey = "vmwarevsphereMachineConfigs"
)

type VmwarevsphereMachineConfigs struct {
	VmwarevsphereMachineConfig []VmwarevsphereMachineConfig `json:"vmwarevsphereMachineConfig" yaml:"vmwarevsphereMachineConfig"`

	Hostsystem   string `json:"hostsystem" yaml:"hostsystem"`
	Datacenter   string `json:"datacenter" yaml:"datacenter"`
	Datastore    string `json:"datastore" yaml:"datastore"`
	DatastoreURL string `json:"datastoreURL" yaml:"datastoreURL"`
	Folder       string `json:"folder" yaml:"folder"`
	Pool         string `json:"pool" yaml:"pool"`
}

// VsphereMachineConfig is configuration needed to create an rke-machine-config.cattle.io.vmwarevsphereconfig
type VmwarevsphereMachineConfig struct {
	Roles
	Boot2dockerURL          string   `json:"boot2dockerURL" yaml:"boot2dockerURL"`
	Cfgparam                []string `json:"cfgparam" yaml:"cfgparam"`
	CloneFrom               string   `json:"cloneFrom" yaml:"cloneFrom"`
	CloudConfig             string   `json:"cloudConfig" yaml:"cloudConfig"`
	Cloundinit              string   `json:"cloundinit" yaml:"cloundinit"`
	ContentLibrary          string   `json:"contentLibrary" yaml:"contentLibrary"`
	CPUCount                string   `json:"cpuCount" yaml:"cpuCount"`
	CreationType            string   `json:"creationType" yaml:"creationType"`
	CustomAttribute         []string `json:"customAttribute" yaml:"customAttribute"`
	DatastoreCluster        string   `json:"datastoreCluster" yaml:"datastoreCluster"`
	DiskSize                string   `json:"diskSize" yaml:"diskSize"`
	MemorySize              string   `json:"memorySize" yaml:"memorySize"`
	Network                 []string `json:"network" yaml:"network"`
	GracefulShutdownTimeout string   `json:"gracefulShutdownTimeout" yaml:"gracefulShutdownTimeout"`
	OS                      string   `json:"os" yaml:"os"`
	SSHPassword             string   `json:"sshPassword" yaml:"sshPassword"`
	SSHPort                 string   `json:"sshPort" yaml:"sshPort"`
	SSHUser                 string   `json:"sshUser" yaml:"sshUser"`
	SSHUserGroup            string   `json:"sshUserGroup" yaml:"sshUserGroup"`
	Tag                     []string `json:"tag" yaml:"tag"`
	VappIpallocationplicy   string   `json:"vappIpallocationplicy" yaml:"vappIpallocationplicy"`
	VappIpprotocol          string   `json:"vappIpprotocol" yaml:"vappIpprotocol"`
	VappProperty            []string `json:"vappProperty" yaml:"vappProperty"`
	VappTransport           string   `json:"vappTransport" yaml:"vappTransport"`
}

// NewVSphereMachineConfig is a constructor to set up rke-machine-config.cattle.io.vmwarevsphereconfig. It returns an *unstructured.Unstructured
// that CreateMachineConfig uses to created the rke-machine-config
func NewVSphereMachineConfig(generatedPoolName, namespace string) []unstructured.Unstructured {
	var vmwarevsphereMachineConfigs VmwarevsphereMachineConfigs
	config.LoadConfig(VmwarevsphereMachineConfigConfigurationFileKey, &vmwarevsphereMachineConfigs)
	var multiConfig []unstructured.Unstructured

	for _, vsphereMachineConfig := range vmwarevsphereMachineConfigs.VmwarevsphereMachineConfig {
		machineConfig := unstructured.Unstructured{}
		machineConfig.SetAPIVersion("rke-machine-config.cattle.io/v1")
		machineConfig.SetKind(VmwaresphereKind)
		machineConfig.SetGenerateName(generatedPoolName)
		machineConfig.SetNamespace(namespace)

		machineConfig.Object["boot2dockerURL"] = vsphereMachineConfig.Boot2dockerURL
		machineConfig.Object["cfgparam"] = vsphereMachineConfig.Cfgparam
		machineConfig.Object["cloneFrom"] = vsphereMachineConfig.CloneFrom
		machineConfig.Object["cloudConfig"] = vsphereMachineConfig.CloudConfig
		machineConfig.Object["cloundinit"] = vsphereMachineConfig.Cloundinit
		machineConfig.Object["contentLibrary"] = vsphereMachineConfig.ContentLibrary
		machineConfig.Object["cpuCount"] = vsphereMachineConfig.CPUCount
		machineConfig.Object["creationType"] = vsphereMachineConfig.CreationType
		machineConfig.Object["customAttribute"] = vsphereMachineConfig.CustomAttribute
		machineConfig.Object["datacenter"] = vmwarevsphereMachineConfigs.Datacenter
		machineConfig.Object["datastore"] = vmwarevsphereMachineConfigs.Datastore
		machineConfig.Object["datastoreCluster"] = vsphereMachineConfig.DatastoreCluster
		machineConfig.Object["datastoreUrl"] = vmwarevsphereMachineConfigs.DatastoreURL
		machineConfig.Object["diskSize"] = vsphereMachineConfig.DiskSize
		machineConfig.Object["folder"] = vmwarevsphereMachineConfigs.Folder
		machineConfig.Object["hostsystem"] = vmwarevsphereMachineConfigs.Hostsystem
		machineConfig.Object["memorySize"] = vsphereMachineConfig.MemorySize
		machineConfig.Object["network"] = vsphereMachineConfig.Network
		machineConfig.Object["os"] = vsphereMachineConfig.OS
		machineConfig.Object["pool"] = vmwarevsphereMachineConfigs.Pool
		machineConfig.Object["sshPassword"] = vsphereMachineConfig.SSHPassword
		machineConfig.Object["sshPort"] = vsphereMachineConfig.SSHPort
		machineConfig.Object["sshUser"] = vsphereMachineConfig.SSHUser
		machineConfig.Object["sshUserGroup"] = vsphereMachineConfig.SSHUserGroup
		machineConfig.Object["tag"] = vsphereMachineConfig.Tag
		machineConfig.Object["vappIpallocationpolicy"] = vsphereMachineConfig.VappIpallocationplicy
		machineConfig.Object["vappIpprotocol"] = vsphereMachineConfig.VappIpprotocol
		machineConfig.Object["vappProperty"] = vsphereMachineConfig.VappProperty
		machineConfig.Object["vappTransport"] = vsphereMachineConfig.VappTransport
		machineConfig.Object["gracefulShutdownTimeout"] = vsphereMachineConfig.GracefulShutdownTimeout

		multiConfig = append(multiConfig, machineConfig)
	}

	return multiConfig
}

func GetVsphereMachineRoles() []Roles {
	var vmwarevsphereMachineConfigs VmwarevsphereMachineConfigs
	config.LoadConfig(VmwarevsphereMachineConfigConfigurationFileKey, &vmwarevsphereMachineConfigs)
	var allRoles []Roles

	for _, vsphereMachineConfig := range vmwarevsphereMachineConfigs.VmwarevsphereMachineConfig {
		allRoles = append(allRoles, vsphereMachineConfig.Roles)
	}

	return allRoles
}
