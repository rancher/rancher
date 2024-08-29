package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rancher/wrangler/v3/pkg/ticker"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/rancher/rancher/pkg/auth/requests/sar"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
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

// NewMetricsHandler configures an HTTP middleware that verifies that the user making the request is allowed to read Rancher metrics
func NewMetricsHandler(scaledContextClient kubernetes.Interface) func(handler http.Handler) http.Handler {
	return sar.NewSubjectAccessReviewHandler(scaledContextClient.AuthorizationV1().SubjectAccessReviews(), &authorizationv1.ResourceAttributes{
		Verb:     "get",
		Resource: "ranchermetrics",
		Group:    "management.cattle.io",
	})
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
