package monitor

import (
	"time"

	promapiv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type TimeSeries struct {
	Name   string            `json:"name"`
	Points [][]float64       `json:"points"`
	Tags   map[string]string `json:"tags"`
}

type PrometheusQuery struct {
	ID              string
	Expr            string
	Step            time.Duration
	Start           time.Time
	End             time.Time
	LegendFormat    string
	IsInstanceQuery bool
}

func (pq *PrometheusQuery) getRange() promapiv1.Range {
	return promapiv1.Range{
		Start: pq.Start,
		End:   pq.End,
		Step:  pq.Step,
	}
}
