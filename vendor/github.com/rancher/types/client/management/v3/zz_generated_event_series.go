package client

const (
	EventSeriesType                  = "eventSeries"
	EventSeriesFieldCount            = "count"
	EventSeriesFieldLastObservedTime = "lastObservedTime"
	EventSeriesFieldState            = "state"
)

type EventSeries struct {
	Count            int64      `json:"count,omitempty" yaml:"count,omitempty"`
	LastObservedTime *MicroTime `json:"lastObservedTime,omitempty" yaml:"lastObservedTime,omitempty"`
	State            string     `json:"state,omitempty" yaml:"state,omitempty"`
}
