package metrics

import (
	"context"
	"time"

	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rancher/wrangler/v3/pkg/ticker"
)

var (
	prometheusMetrics = false

	gcInterval = 60 * time.Second

	clusterOwner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cluster_manager",
			Name:      "cluster_owner",
			Help:      "Set a cluster owner to determine which Rancher server is running a clusters controllers",
		},
		[]string{"cluster", "owner"},
	)
)

type ClusterCounts struct {
	AppRevisions            int `json:"appRevisions"`
	CatalogTemplateVersions int `json:"catalogTemplateVersions"`
	Projects                int `json:"projects"`
	ConfigMaps              int `json:"configMaps"`
	Secrets                 int `json:"secrets"`
	Namespaces              int `json:"namespaces"`
	Nodes                   int `json:"nodes"`
	RoleBindings            int `json:"roleBindings"`
	ClusterRoleBindings     int `json:"clusterRoleBindings"`
}

func Register(ctx context.Context, scaledContext *config.ScaledContext) {
	prometheusMetrics = true

	// Cluster Owner
	prometheus.MustRegister(clusterOwner)

	// node and node core metrics
	prometheus.MustRegister(numNodes)
	prometheus.MustRegister(numCores)

	gc := metricGarbageCollector{
		clusterLister:  scaledContext.Management.Clusters("").Controller().Lister(),
		nodeLister:     scaledContext.Management.Nodes("").Controller().Lister(),
		endpointLister: scaledContext.Core.Endpoints(settings.Namespace.Get()).Controller().Lister(),
	}

	nm := &nodeMetrics{
		nodeCache:    scaledContext.Wrangler.Mgmt.Node().Cache(),
		clusterCache: scaledContext.Wrangler.Mgmt.Cluster().Cache(),
	}

	go func(ctx context.Context) {
		for range ticker.Context(ctx, gcInterval) {
			gc.metricGarbageCollection()
		}
	}(ctx)

	go nm.collect(ctx)
}

func SetClusterOwner(id, clusterID string) {
	if prometheusMetrics {
		clusterOwner.With(
			prometheus.Labels{
				"cluster": clusterID,
				"owner":   id,
			}).Set(float64(1))
	}
}

func UnsetClusterOwner(id, clusterID string) {
	if prometheusMetrics {
		clusterOwner.With(
			prometheus.Labels{
				"cluster": clusterID,
				"owner":   id,
			}).Set(float64(0))
	}
}
