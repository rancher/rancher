package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/rancher/pkg/auth/util"
	"github.com/rancher/rancher/pkg/clustermanager"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/rancher/pkg/types/config"
	"github.com/rancher/wrangler/v2/pkg/ticker"
	authV1 "k8s.io/api/authorization/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clusterManager *clustermanager.Manager
	k8sClient      kubernetes.Interface
	next           http.Handler
}

type ClusterCounts struct {
	AppRevisions            int64 `json:"appRevisions"`
	CatalogTemplateVersions int64 `json:"catalogTemplateVersions"`
	Projects                int64 `json:"projects"`
	ConfigMaps              int64 `json:"configMaps"`
	Secrets                 int64 `json:"secrets"`
	Namespaces              int64 `json:"namespaces"`
	Nodes                   int64 `json:"nodes"`
	RoleBindings            int64 `json:"roleBindings"`
	ClusterRoleBindings     int64 `json:"clusterRoleBindings"`
}

func NewMetricsHandler(scaledContext *config.ScaledContext, clusterManager *clustermanager.Manager, handler http.Handler) http.Handler {
	return &metricsHandler{
		clusterManager: clusterManager,
		k8sClient:      scaledContext.K8sClient,
		next:           handler,
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

	result, err := h.k8sClient.AuthorizationV1().SubjectAccessReviews().Create(req.Context(), &review, metav1.CreateOptions{})
	if err != nil {
		util.ReturnHTTPError(rw, req, 500, err.Error())
		return
	}

	if !result.Status.Allowed {
		util.ReturnHTTPError(rw, req, 401, "Unauthorized")
		return
	}

	if id := mux.Vars(req)["clusterID"]; id != "" {
		h.getClusterObjectCount(req.Context(), id, rw, req)
		return
	}

	h.next.ServeHTTP(rw, req)
}

// getClusterObjectCount uses the caches to get the number of items in a cluster and return a json blob with this
// information. The count is based off the cluster itself and where the object lives, not the count as you would
// see through the UI. For example projects only live in the management cluster so the count would only be displayed
// if getting 'local'. The count would show as 0 for a downstream cluster since the CR that backs the project only
// lives in management.
func (h *metricsHandler) getClusterObjectCount(ctx context.Context, clusterID string, rw http.ResponseWriter, req *http.Request) {
	cluster, err := h.getClusterClient(clusterID)
	if err != nil {
		var (
			normanError *httperror.APIError
			k8sError    *k8serrors.StatusError
			code        int
			message     string
		)

		if errors.As(err, &normanError) {
			code = normanError.Code.Status
			message = normanError.Message
		} else if errors.As(err, &k8sError) {
			code = int(k8sError.ErrStatus.Code)
			message = k8sError.Error()
		}
		util.ReturnHTTPError(rw, req, code, message)
		return
	}

	var cc ClusterCounts

	cc.AppRevisions, err = cluster.AppRevisions(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	// None of these resources are pushed downstream to a user cluster so we can only count them in the management cluster.
	if clusterID == "local" {
		cc.CatalogTemplateVersions, err = cluster.CatalogTemplateVersions(ctx)
		if err != nil {
			returnK8serror(err, rw, req)
			return
		}

		cc.Projects, err = cluster.Projects(ctx)
		if err != nil {
			returnK8serror(err, rw, req)
			return
		}
	}

	cc.ConfigMaps, err = cluster.ConfigMaps(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	cc.Secrets, err = cluster.Secrets(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	cc.Namespaces, err = cluster.Namespaces(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	cc.Nodes, err = cluster.Nodes(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	cc.RoleBindings, err = cluster.RoleBindings(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	cc.ClusterRoleBindings, err = cluster.ClusterRoleBindings(ctx)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}

	js, err := json.Marshal(cc)
	if err != nil {
		util.ReturnHTTPError(rw, req, 500, err.Error())
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(js)
}

// returnK8serror attempts to respond using the k8s error and its code/message
func returnK8serror(err error, rw http.ResponseWriter, req *http.Request) {
	var k8sError *k8serrors.StatusError
	if errors.As(err, &k8sError) {
		util.ReturnHTTPError(rw, req, int(k8sError.ErrStatus.Code), k8sError.Error())
		return
	}
	// Well, it wasn't a k8s error, give it back
	util.ReturnHTTPError(rw, req, 500, err.Error())
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
