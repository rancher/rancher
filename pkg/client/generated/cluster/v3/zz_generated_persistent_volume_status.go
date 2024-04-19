package client

const (
	PersistentVolumeStatusType                         = "persistentVolumeStatus"
	PersistentVolumeStatusFieldLastPhaseTransitionTime = "lastPhaseTransitionTime"
	PersistentVolumeStatusFieldMessage                 = "message"
	PersistentVolumeStatusFieldPhase                   = "phase"
	PersistentVolumeStatusFieldReason                  = "reason"
)

type PersistentVolumeStatus struct {
	LastPhaseTransitionTime string `json:"lastPhaseTransitionTime,omitempty" yaml:"lastPhaseTransitionTime,omitempty"`
	Message                 string `json:"message,omitempty" yaml:"message,omitempty"`
	Phase                   string `json:"phase,omitempty" yaml:"phase,omitempty"`
	Reason                  string `json:"reason,omitempty" yaml:"reason,omitempty"`
}
