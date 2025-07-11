package client

const (
	PodConditionType                    = "podCondition"
	PodConditionFieldLastProbeTime      = "lastProbeTime"
	PodConditionFieldLastTransitionTime = "lastTransitionTime"
	PodConditionFieldMessage            = "message"
	PodConditionFieldObservedGeneration = "observedGeneration"
	PodConditionFieldReason             = "reason"
	PodConditionFieldStatus             = "status"
	PodConditionFieldType               = "type"
)

type PodCondition struct {
	LastProbeTime      string `json:"lastProbeTime,omitempty" yaml:"lastProbeTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	ObservedGeneration int64  `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
