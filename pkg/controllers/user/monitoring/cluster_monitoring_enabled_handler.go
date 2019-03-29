package monitoring

import (
	"github.com/rancher/rancher/pkg/monitoring"
	mgmtv3 "github.com/rancher/types/apis/management.cattle.io/v3"
	k8scorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// clusterMonitoringEnabledHandler syncs the Cluster Monitoring Prometheus status.
type clusterMonitoringEnabledHandler struct {
	clusterName             string
	cattleClusterController mgmtv3.ClusterController
	cattleClusterLister     mgmtv3.ClusterLister
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
