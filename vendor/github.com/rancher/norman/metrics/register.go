package metrics

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	if os.Getenv(MetricsGenericControllerEnv) == "true" {
		genericControllerMetrics = true
		prometheus.MustRegister(TotalHandlerExecution)
		prometheus.MustRegister(TotalHandlerFailure)
	}
	if os.Getenv(MetricsSessionServerEnv) == "true" {
		sessionServerMetrics = true
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
