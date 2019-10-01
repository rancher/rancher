package metrics

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

const metricsEnv = "CATTLE_PROMETHEUS_METRICS"

var prometheusMetrics = false

func init() {
	if os.Getenv(metricsEnv) != "" {
		prometheusMetrics = true
		// Generic controller metrics
		prometheus.MustRegister(TotalHandlerExecution)
		prometheus.MustRegister(TotalHandlerFailure)

		// Session metrics
		prometheus.MustRegister(TotalAddWS)
		prometheus.MustRegister(TotalRemoveWS)
		prometheus.MustRegister(TotalAddConnectionsForWS)
		prometheus.MustRegister(TotalRemoveConnectionsForWS)
		prometheus.MustRegister(TotalTransmitBytesOnWS)
		prometheus.MustRegister(TotalTransmitErrorBytesOnWS)
		prometheus.MustRegister(TotalReceiveBytesOnWS)
		prometheus.MustRegister(TotalAddPeerAttempt)
		prometheus.MustRegister(TotalPeerConnected)
		prometheus.MustRegister(TotalPeerDisConnected)
	}
}
