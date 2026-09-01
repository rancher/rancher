package client

const (
	VolumeHealthConditionType         = "volumeHealthCondition"
	VolumeHealthConditionFieldMessage = "message"
	VolumeHealthConditionFieldReason  = "reason"
	VolumeHealthConditionFieldStatus  = "status"
)

type VolumeHealthCondition struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	Reason  string `json:"reason,omitempty" yaml:"reason,omitempty"`
	Status  string `json:"status,omitempty" yaml:"status,omitempty"`
}
