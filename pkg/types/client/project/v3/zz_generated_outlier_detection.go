package client

const (
	OutlierDetectionType                    = "outlierDetection"
	OutlierDetectionFieldBaseEjectionTime   = "baseEjectionTime"
	OutlierDetectionFieldConsecutiveErrors  = "consecutiveErrors"
	OutlierDetectionFieldInterval           = "interval"
	OutlierDetectionFieldMaxEjectionPercent = "maxEjectionPercent"
)

type OutlierDetection struct {
	BaseEjectionTime   string `json:"baseEjectionTime,omitempty" yaml:"baseEjectionTime,omitempty"`
	ConsecutiveErrors  int64  `json:"consecutiveErrors,omitempty" yaml:"consecutiveErrors,omitempty"`
	Interval           string `json:"interval,omitempty" yaml:"interval,omitempty"`
	MaxEjectionPercent int64  `json:"maxEjectionPercent,omitempty" yaml:"maxEjectionPercent,omitempty"`
}
