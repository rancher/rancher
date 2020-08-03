package client

const (
	VolumeNodeAffinityType          = "volumeNodeAffinity"
	VolumeNodeAffinityFieldRequired = "required"
)

type VolumeNodeAffinity struct {
	Required *NodeSelector `json:"required,omitempty" yaml:"required,omitempty"`
}
