package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	TotalHandlerExecution = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "norman_generic_controller",
			Name:      "total_handler_execution",
			Help:      "Total count of hanlder executions",
		},
		[]string{"name", "handlerName"},
	)

	TotalHandlerFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "norman_generic_controller",
			Name:      "total_handler_failure",
			Help:      "Total count of handler failures",
		},
		[]string{"name", "handlerName", "key"},
	)
)

func IncTotalHandlerExecution(controllerName, handlerName string) {
	if prometheusMetrics {
		TotalHandlerExecution.With(
			prometheus.Labels{
				"name":        controllerName,
				"handlerName": handlerName},
		).Inc()
	}
}

func IncTotalHandlerFailure(controllerName, handlerName, key string) {
	if prometheusMetrics {
		TotalHandlerFailure.With(
			prometheus.Labels{
				"name":        controllerName,
				"handlerName": handlerName,
				"key":         key,
			},
		).Inc()
	}
}
