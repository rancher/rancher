package client

const (
	MachineDeletionStatusType                                  = "machineDeletionStatus"
	MachineDeletionStatusFieldNodeDrainStartTime               = "nodeDrainStartTime"
	MachineDeletionStatusFieldWaitForNodeVolumeDetachStartTime = "waitForNodeVolumeDetachStartTime"
	MachineDeletionStatusFieldWaitForPreDrainHookStartTime     = "waitForPreDrainHookStartTime"
	MachineDeletionStatusFieldWaitForPreTerminateHookStartTime = "waitForPreTerminateHookStartTime"
)

type MachineDeletionStatus struct {
	NodeDrainStartTime               string `json:"nodeDrainStartTime,omitempty" yaml:"nodeDrainStartTime,omitempty"`
	WaitForNodeVolumeDetachStartTime string `json:"waitForNodeVolumeDetachStartTime,omitempty" yaml:"waitForNodeVolumeDetachStartTime,omitempty"`
	WaitForPreDrainHookStartTime     string `json:"waitForPreDrainHookStartTime,omitempty" yaml:"waitForPreDrainHookStartTime,omitempty"`
	WaitForPreTerminateHookStartTime string `json:"waitForPreTerminateHookStartTime,omitempty" yaml:"waitForPreTerminateHookStartTime,omitempty"`
}
