package monitoring

import (
	"github.com/pkg/errors"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	mgmtv3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/monitoring"
	k8scorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

// clusterMonitoringEnabledHandler syncs the Cluster Monitoring Prometheus status.
type clusterMonitoringEnabledHandler struct {
	clusterName             string
	cattleClusterController mgmtv3.ClusterController
	cattleClusterLister     mgmtv3.ClusterLister
	agentEndpointsLister    corev1.EndpointsLister
}

// sync will trigger clusterHandler sync loop, if Cluster Monitoring is enabling.
func (h *clusterMonitoringEnabledHandler) sync(key string, endpoints *k8scorev1.Endpoints) (runtime.Object, error) {
	endpointName, _, _ := monitoring.ClusterPrometheusEndpoint()
	if endpoints == nil || endpoints.DeletionTimestamp != nil || endpoints.Name != endpointName {
		return endpoints, nil
	}

	cluster, err := h.cattleClusterLister.Get(metav1.NamespaceAll, h.clusterName)
	if err != nil || cluster.DeletionTimestamp != nil {
		return endpoints, err
	}

	// only consider enabling monitoring
	if !cluster.Spec.EnableClusterMonitoring {
		return endpoints, nil
	}

	// trigger clusterHandler sync loop
	h.cattleClusterController.Enqueue(metav1.NamespaceAll, h.clusterName)
	return endpoints, nil
}

// sync will trigger clusterHandler sync loop when node updated, if Cluster Monitoring is enabling.
func (h *clusterMonitoringEnabledHandler) syncWindowsNode(key string, node *k8scorev1.Node) (runtime.Object, error) {
	if node == nil || node.Labels == nil {
		return node, nil
	}

	_, monitoringNamespace := monitoring.ClusterMonitoringInfo()
	cluster, err := h.cattleClusterLister.Get(metav1.NamespaceAll, h.clusterName)
	if err != nil || cluster.DeletionTimestamp != nil {
		return node, err
	}

	// only consider enabling monitoring
	if !cluster.Spec.EnableClusterMonitoring {
		return node, nil
	}

	if !windowNodeLabel.Matches(labels.Set(node.Labels)) {
		return node, nil
	}

	endpoint, err := h.agentEndpointsLister.Get(monitoringNamespace, nodeMetricsWindowsEndpointName)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, "get windows endpoint %s:%s failed", monitoringNamespace, nodeMetricsWindowsEndpointName)
	}
	// enqueue to enable windows node exporter
	if endpoint == nil {
		h.cattleClusterController.Enqueue(metav1.NamespaceAll, h.clusterName)
	}

	return node, nil
}
