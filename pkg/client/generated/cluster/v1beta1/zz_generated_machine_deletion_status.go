package client

const (
	MachineDeletionStatusType                                  = "machineDeletionStatus"
	MachineDeletionStatusFieldNodeDrainStartTime               = "nodeDrainStartTime"
	MachineDeletionStatusFieldWaitForNodeVolumeDetachStartTime = "waitForNodeVolumeDetachStartTime"
)

type MachineDeletionStatus struct {
	NodeDrainStartTime               string `json:"nodeDrainStartTime,omitempty" yaml:"nodeDrainStartTime,omitempty"`
	WaitForNodeVolumeDetachStartTime string `json:"waitForNodeVolumeDetachStartTime,omitempty" yaml:"waitForNodeVolumeDetachStartTime,omitempty"`
}
