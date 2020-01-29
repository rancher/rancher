package metrics

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

const metricsEnv = "CATTLE_PROMETHEUS_METRICS"

var prometheusMetrics = false

func init() {
	if os.Getenv(metricsEnv) == "true" {
		prometheusMetrics = true
		// Generic controller metrics
		prometheus.MustRegister(TotalHandlerExecution)
		prometheus.MustRegister(TotalHandlerFailure)
	}
}
