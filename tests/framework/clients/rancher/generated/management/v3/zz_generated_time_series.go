package client

const (
	TimeSeriesType        = "timeSeries"
	TimeSeriesFieldName   = "name"
	TimeSeriesFieldPoints = "points"
)

type TimeSeries struct {
	Name   string      `json:"name,omitempty" yaml:"name,omitempty"`
	Points [][]float64 `json:"points,omitempty" yaml:"points,omitempty"`
}
