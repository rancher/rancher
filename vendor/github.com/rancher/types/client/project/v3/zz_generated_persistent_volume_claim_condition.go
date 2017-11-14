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
	LastProbeTime      string `json:"lastProbeTime,omitempty"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Message            string `json:"message,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Status             string `json:"status,omitempty"`
	Type               string `json:"type,omitempty"`
}
