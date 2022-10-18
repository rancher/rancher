package client

const (
	MachineGlobalConfigType     = "machineGlobalConfig"
	MachineGlobalConfigFieldCNI = "cni"
)

type MachineGlobalConfig struct {
	CNI string `json:"cni,omitempty" yaml:"cni,omitempty"`
}
