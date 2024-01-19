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
	"github.com/rancher/wrangler/pkg/ticker"
	authV1 "k8s.io/api/authorization/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
		h.getClusterObjectCount(id, rw, req)
		return
	}

	h.next.ServeHTTP(rw, req)
}

// getClusterObjectCount uses the caches to get the number of items in a cluster and return a json blob with this
// information. The count is based off the cluster itself and where the object lives, not the count as you would
// see through the UI. For example projects only live in the management cluster so the count would only be displayed
// if getting 'local'. The count would show as 0 for a downstream cluster since the CR that backs the project only
// lives in management.
func (h *metricsHandler) getClusterObjectCount(clusterID string, rw http.ResponseWriter, req *http.Request) {
	cluster, err := h.clusterManager.UserContext(clusterID)
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
	labelSelector := labels.NewSelector()

	ar, err := cluster.Project.AppRevisions(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.AppRevisions = len(ar)

	// None of these resources are pushed downstream to a user cluster so we can only count them in the management cluster.
	if clusterID == "local" {
		ctv, err := cluster.Management.Management.CatalogTemplateVersions(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
		if err != nil {
			returnK8serror(err, rw, req)
			return
		}
		cc.CatalogTemplateVersions = len(ctv)

		projects, err := cluster.Management.Management.Projects(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
		if err != nil {
			returnK8serror(err, rw, req)
			return
		}
		cc.Projects = len(projects)
	}

	cf, err := cluster.Core.ConfigMaps(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.ConfigMaps = len(cf)

	s, err := cluster.Core.Secrets(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.Secrets = len(s)

	ns, err := cluster.Core.Namespaces(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.Namespaces = len(ns)

	nodes, err := cluster.Core.Nodes(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.Nodes = len(nodes)

	rb, err := cluster.RBAC.RoleBindings(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.RoleBindings = len(rb)

	crb, err := cluster.RBAC.ClusterRoleBindings(metav1.NamespaceAll).Controller().Lister().List(metav1.NamespaceAll, labelSelector)
	if err != nil {
		returnK8serror(err, rw, req)
		return
	}
	cc.ClusterRoleBindings = len(crb)

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

	// cache metrics
	prometheus.MustRegister(
		config.DeferredCachesCounter,
		config.DeferredCachesActiveCounter)

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
