package metrics

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
)

const metricsEnv = "CATTLE_PROMETHEUS_METRICS"

var prometheusMetrics = false

var (
	TotalAddWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_add_websocket_session",
			Help:      "Total count of added websocket sessions",
		},
		[]string{"clientkey", "peer"})

	TotalRemoveWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_remove_websocket_session",
			Help:      "Total count of removed websocket sessions",
		},
		[]string{"clientkey", "peer"})

	TotalAddConnectionsForWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_add_connections",
			Help:      "Total count of added connections",
		},
		[]string{"clientkey", "proto", "addr"},
	)

	TotalRemoveConnectionsForWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_remove_connections",
			Help:      "Total count of removed connections",
		},
		[]string{"clientkey", "proto", "addr"},
	)

	TotalTransmitBytesOnWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_transmit_bytes",
			Help:      "Total bytes transmitted",
		},
		[]string{"clientkey"},
	)

	TotalTransmitErrorBytesOnWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_transmit_error_bytes",
			Help:      "Total error bytes transmitted",
		},
		[]string{"clientkey"},
	)

	TotalReceiveBytesOnWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_receive_bytes",
			Help:      "Total bytes received",
		},
		[]string{"clientkey"},
	)

	TotalAddPeerAttempt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_peer_ws_attempt",
			Help:      "Total count of attempts to establish websocket session to other rancher-server",
		},
		[]string{"peer"},
	)
	TotalPeerConnected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_peer_ws_connected",
			Help:      "Total count of connected websocket sessions to other rancher-server",
		},
		[]string{"peer"},
	)
	TotalPeerDisConnected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_peer_ws_disconnected",
			Help:      "Total count of disconnected websocket sessions from other rancher-server",
		},
		[]string{"peer"},
	)
)

// Register registers a series of session
// metrics for Prometheus.
func Register() {

	prometheusMetrics = true

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

func init() {
	if os.Getenv(metricsEnv) == "true" {
		Register()
	}
}

func IncSMTotalAddWS(clientKey string, peer bool) {
	var peerStr string
	if peer {
		peerStr = "true"
	} else {
		peerStr = "false"
	}
	if prometheusMetrics {
		TotalAddWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"peer":      peerStr,
			}).Inc()
	}
}

func IncSMTotalRemoveWS(clientKey string, peer bool) {
	var peerStr string
	if prometheusMetrics {
		if peer {
			peerStr = "true"
		} else {
			peerStr = "false"
		}
		TotalRemoveWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"peer":      peerStr,
			}).Inc()
	}
}

func AddSMTotalTransmitErrorBytesOnWS(clientKey string, size float64) {
	if prometheusMetrics {
		TotalTransmitErrorBytesOnWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
			}).Add(size)
	}
}

func AddSMTotalTransmitBytesOnWS(clientKey string, size float64) {
	if prometheusMetrics {
		TotalTransmitBytesOnWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
			}).Add(size)
	}
}

func AddSMTotalReceiveBytesOnWS(clientKey string, size float64) {
	if prometheusMetrics {
		TotalReceiveBytesOnWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
			}).Add(size)
	}
}

func IncSMTotalAddConnectionsForWS(clientKey, proto, addr string) {
	if prometheusMetrics {
		TotalAddConnectionsForWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"proto":     proto,
				"addr":      addr,
			}).Inc()
	}
}

func IncSMTotalRemoveConnectionsForWS(clientKey, proto, addr string) {
	if prometheusMetrics {
		TotalRemoveConnectionsForWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"proto":     proto,
				"addr":      addr,
			}).Inc()
	}
}

func IncSMTotalAddPeerAttempt(peer string) {
	if prometheusMetrics {
		TotalAddPeerAttempt.With(
			prometheus.Labels{
				"peer": peer,
			}).Inc()
	}
}

func IncSMTotalPeerConnected(peer string) {
	if prometheusMetrics {
		TotalPeerConnected.With(
			prometheus.Labels{
				"peer": peer,
			}).Inc()
	}
}

func IncSMTotalPeerDisConnected(peer string) {
	if prometheusMetrics {
		TotalPeerDisConnected.With(
			prometheus.Labels{
				"peer": peer,
			}).Inc()

	}
}
