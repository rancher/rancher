package schema

type Duration struct {
	Duration string `json:"duration,omitempty" yaml:"duration,omitempty"`
}

type MachineGlobalConfig struct {
	CNI string `json:"cni,omitempty" yaml:"cni,omitempty"`
}
