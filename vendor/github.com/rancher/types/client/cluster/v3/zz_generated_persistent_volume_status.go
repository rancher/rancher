package client

const (
	PersistentVolumeStatusType         = "persistentVolumeStatus"
	PersistentVolumeStatusFieldMessage = "message"
	PersistentVolumeStatusFieldPhase   = "phase"
	PersistentVolumeStatusFieldReason  = "reason"
)

type PersistentVolumeStatus struct {
	Message string `json:"message,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
