package client

const (
	PersistentVolumeClaimConditionType                    = "persistentVolumeClaimCondition"
	PersistentVolumeClaimConditionFieldLastProbeTime      = "lastProbeTime"
	PersistentVolumeClaimConditionFieldLastTransitionTime = "lastTransitionTime"
	PersistentVolumeClaimConditionFieldMessage            = "message"
	PersistentVolumeClaimConditionFieldReason             = "reason"
	PersistentVolumeClaimConditionFieldStatus             = "status"
	PersistentVolumeClaimConditionFieldType               = "type"
)

type PersistentVolumeClaimCondition struct {
	LastProbeTime      string `json:"lastProbeTime,omitempty" yaml:"lastProbeTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason             string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status             string `json:"status,omitempty" yaml:"status,omitempty"`
	Type               string `json:"type,omitempty" yaml:"type,omitempty"`
}
