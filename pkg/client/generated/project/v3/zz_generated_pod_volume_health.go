package client

const (
	PodVolumeHealthType                    = "podVolumeHealth"
	PodVolumeHealthFieldHealthConditions   = "healthConditions"
	PodVolumeHealthFieldLastTransitionTime = "lastTransitionTime"
	PodVolumeHealthFieldName               = "name"
)

type PodVolumeHealth struct {
	HealthConditions   []VolumeHealthCondition `json:"healthConditions,omitempty" yaml:"healthConditions,omitempty"`
	LastTransitionTime string                  `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Name               string                  `json:"name,omitempty" yaml:"name,omitempty"`
}
