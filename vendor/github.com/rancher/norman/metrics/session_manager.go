package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const MetricsSessionServerEnv = "NORMAN_SESSION_SERVER_METRICS"

var (
	sessionServerMetrics = false
	TotalAddWS           = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_add_websocket_session",
			Help:      "Total Count of adding websocket session",
		},
		[]string{"clientkey", "peer"})

	TotalRemoveWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_remove_websocket_session",
			Help:      "Total Count of removing websocket session",
		},
		[]string{"clientkey", "peer"})

	TotalAddConnectionsForWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_add_connections",
			Help:      "Total count of adding connection",
		},
		[]string{"clientkey", "proto", "addr"},
	)

	TotalRemoveConnectionsForWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_remove_connections",
			Help:      "Total count of removing connection",
		},
		[]string{"clientkey", "proto", "addr"},
	)

	TotalTransmitBytesOnWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_transmit_bytes",
			Help:      "Total bytes of transmiting",
		},
		[]string{"clientkey"},
	)

	TotalTransmitErrorBytesOnWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_transmit_error_bytes",
			Help:      "Total bytes of transmiting error",
		},
		[]string{"clientkey"},
	)

	TotalReceiveBytesOnWS = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_receive_bytes",
			Help:      "Total bytes of receiving",
		},
		[]string{"clientkey"},
	)

	TotalAddPeerAttempt = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_peer_ws_attempt",
			Help:      "Total count of attempt to establish websocket session to other rancher-server",
		},
		[]string{"peer"},
	)
	TotalPeerConnected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_peer_ws_connected",
			Help:      "Total count of connected websocket session to other rancher-server",
		},
		[]string{"peer"},
	)
	TotalPeerDisConnected = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "session_server",
			Name:      "total_peer_ws_disconnected",
			Help:      "Total count of dis-connected websocket session from other rancher-server",
		},
		[]string{"peer"},
	)
)

func IncSMTotalAddWS(clientKey string, peer bool) {
	var peerStr string
	if peer {
		peerStr = "true"
	} else {
		peerStr = "false"
	}
	if sessionServerMetrics {
		TotalAddWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"peer":      peerStr,
			}).Inc()
	}
}

func IncSMTotalRemoveWS(clientKey string, peer bool) {
	var peerStr string
	if sessionServerMetrics {
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
	if sessionServerMetrics {
		TotalTransmitErrorBytesOnWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
			}).Add(size)
	}
}

func AddSMTotalTransmitBytesOnWS(clientKey string, size float64) {
	if sessionServerMetrics {
		TotalTransmitBytesOnWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
			}).Add(size)
	}
}

func AddSMTotalReceiveBytesOnWS(clientKey string, size float64) {
	if sessionServerMetrics {
		TotalReceiveBytesOnWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
			}).Add(size)
	}
}

func IncSMTotalAddConnectionsForWS(clientKey, proto, addr string) {
	if sessionServerMetrics {
		TotalAddConnectionsForWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"proto":     proto,
				"addr":      addr}).Inc()
	}
}

func IncSMTotalRemoveConnectionsForWS(clientKey, proto, addr string) {
	if sessionServerMetrics {
		TotalRemoveConnectionsForWS.With(
			prometheus.Labels{
				"clientkey": clientKey,
				"proto":     proto,
				"addr":      addr}).Inc()
	}
}

func IncSMTotalAddPeerAttempt(peer string) {
	if sessionServerMetrics {
		TotalAddPeerAttempt.With(
			prometheus.Labels{
				"peer": peer,
			}).Inc()
	}
}

func IncSMTotalPeerConnected(peer string) {
	if sessionServerMetrics {
		TotalPeerConnected.With(
			prometheus.Labels{
				"peer": peer,
			}).Inc()
	}
}

func IncSMTotalPeerDisConnected(peer string) {
	if sessionServerMetrics {
		TotalPeerDisConnected.With(
			prometheus.Labels{
				"peer": peer,
			}).Inc()

	}
}
