package client

const (
	YAxisType      = "yAxis"
	YAxisFieldUnit = "unit"
)

type YAxis struct {
	Unit string `json:"unit,omitempty" yaml:"unit,omitempty"`
}
