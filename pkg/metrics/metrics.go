package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rancher/types/config"
)

var (
	PrometheusMetrics = false

	GCInterval = time.Duration(60)

	ClusterOwner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cluster_manager",
			Name:      "cluster_owner",
			Help:      "Set 1 when cluster is owned, by this metrics you can find out which rancher server run user controllers for specific cluster",
		},
		[]string{"cluster", "owner"},
	)
)

func Register(context *config.ScaledContext) {
	PrometheusMetrics = true

	// Cluster Owner
	prometheus.MustRegister(ClusterOwner)

	go func() {
		for {
			time.Sleep(GCInterval * time.Second)
			MetricGarbageCollection(context)
		}
	}()
}

func SetClusterOwner(id, clusterID string) {
	if PrometheusMetrics {
		ClusterOwner.With(
			prometheus.Labels{
				"cluster": clusterID,
				"owner":   id,
			}).Set(float64(1))
	}
}

func UnsetClusterOwner(id, clusterID string) {
	if PrometheusMetrics {
		ClusterOwner.With(
			prometheus.Labels{
				"cluster": clusterID,
				"owner":   id,
			}).Set(float64(0))
	}
}
