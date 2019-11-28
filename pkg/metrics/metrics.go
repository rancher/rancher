package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/ticker"
	"github.com/rancher/types/config"
	authV1 "k8s.io/api/authorization/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	prometheusMetrics = false

	gcInterval = time.Duration(60 * time.Second)

	clusterOwner = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cluster_manager",
			Name:      "cluster_owner",
			Help:      "Set a cluster owner to determine which Rancher server is running a clusters controllers",
		},
		[]string{"cluster", "owner"},
	)
)

type metricsHandler struct {
	k8sClient kubernetes.Interface
	next      http.Handler
}

func NewMetricsHandler(scaledContext *config.ScaledContext, handler http.Handler) http.Handler {
	return &metricsHandler{
		k8sClient: scaledContext.K8sClient,
		next:      handler,
	}
}

func (h *metricsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	var reqGroup []string
	if g, ok := req.Header["Impersonate-Group"]; ok {
		reqGroup = g
	}

	review := authV1.SubjectAccessReview{
		Spec: authV1.SubjectAccessReviewSpec{
			User:   req.Header.Get("Impersonate-User"),
			Groups: reqGroup,
			ResourceAttributes: &authV1.ResourceAttributes{
				Verb:     "get",
				Resource: "ranchermetrics",
				Group:    "management.cattle.io",
			},
		},
	}

	result, err := h.k8sClient.AuthorizationV1().SubjectAccessReviews().Create(&review)
	if err != nil {
		util.ReturnHTTPError(rw, req, 500, err.Error())
		return
	}

	if !result.Status.Allowed {
		util.ReturnHTTPError(rw, req, 401, "Unauthorized")
		return
	}

	h.next.ServeHTTP(rw, req)
}

func Register(ctx context.Context, scaledContext *config.ScaledContext) {
	prometheusMetrics = true

	// Cluster Owner
	prometheus.MustRegister(clusterOwner)

	gc := metricGarbageCollector{
		clusterLister:  scaledContext.Management.Clusters("").Controller().Lister(),
		nodeLister:     scaledContext.Management.Nodes("").Controller().Lister(),
		endpointLister: scaledContext.Core.Endpoints(settings.Namespace.Get()).Controller().Lister(),
	}

	go func(ctx context.Context) {
		for range ticker.Context(ctx, gcInterval) {
			gc.metricGarbageCollection()
		}
	}(ctx)
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
