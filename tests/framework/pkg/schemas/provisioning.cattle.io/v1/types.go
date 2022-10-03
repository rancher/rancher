package schema

// MachineGlobalConfig is struct that defines the CNI for a RKEConfig need to provision a v1 cluster
type MachineGlobalConfig struct {
	CNI string `json:"cni,omitempty" yaml:"cni,omitempty"`
}
