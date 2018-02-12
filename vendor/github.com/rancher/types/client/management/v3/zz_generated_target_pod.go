package client

const (
	TargetPodType                        = "targetPod"
	TargetPodFieldCondition              = "condition"
	TargetPodFieldPodId                  = "podId"
	TargetPodFieldRestartIntervalSeconds = "restartIntervalSeconds"
	TargetPodFieldRestartTimes           = "restartTimes"
)

type TargetPod struct {
	Condition              string `json:"condition,omitempty"`
	PodId                  string `json:"podId,omitempty"`
	RestartIntervalSeconds *int64 `json:"restartIntervalSeconds,omitempty"`
	RestartTimes           *int64 `json:"restartTimes,omitempty"`
}
