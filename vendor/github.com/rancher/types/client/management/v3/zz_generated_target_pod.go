package client

const (
	TargetPodType                        = "targetPod"
	TargetPodFieldCondition              = "condition"
	TargetPodFieldPodId                  = "podId"
	TargetPodFieldRestartIntervalSeconds = "restartIntervalSeconds"
	TargetPodFieldRestartTimes           = "restartTimes"
)

type TargetPod struct {
	Condition              string `json:"condition,omitempty" yaml:"condition,omitempty"`
	PodId                  string `json:"podId,omitempty" yaml:"podId,omitempty"`
	RestartIntervalSeconds *int64 `json:"restartIntervalSeconds,omitempty" yaml:"restartIntervalSeconds,omitempty"`
	RestartTimes           *int64 `json:"restartTimes,omitempty" yaml:"restartTimes,omitempty"`
}
