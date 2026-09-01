package client

const (
	VolumeHealthStatusType                    = "volumeHealthStatus"
	VolumeHealthStatusFieldHealthConditions   = "healthConditions"
	VolumeHealthStatusFieldLastTransitionTime = "lastTransitionTime"
)

type VolumeHealthStatus struct {
	HealthConditions   []VolumeHealthCondition `json:"healthConditions,omitempty" yaml:"healthConditions,omitempty"`
	LastTransitionTime string                  `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
}
