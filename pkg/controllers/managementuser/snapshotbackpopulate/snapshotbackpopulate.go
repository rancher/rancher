package snapshotbackpopulate

import (
	"context"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	cluster2 "github.com/rancher/rancher/pkg/controllers/provisioningv2/cluster"
	provisioningcontrollers "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/types/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
)

type handler struct {
	clusterName  string
	clusterCache provisioningcontrollers.ClusterCache
	clusters     provisioningcontrollers.ClusterClient
}

func Register(ctx context.Context, userContext *config.UserContext) {
	h := handler{
		clusterName:  userContext.ClusterName,
		clusterCache: userContext.Management.Wrangler.Provisioning.Cluster().Cache(),
		clusters:     userContext.Management.Wrangler.Provisioning.Cluster(),
	}
	userContext.Core.ConfigMaps("kube-system").AddHandler(ctx, "snapshotbackpopulate", h.OnChange)
}

func (h *handler) OnChange(key string, configMap *corev1.ConfigMap) (runtime.Object, error) {
	if configMap == nil {
		return nil, nil
	}

	if configMap.Namespace != "kube-system" ||
		configMap.Name != "k3s-etcd-snapshots" {
		return configMap, nil
	}

	cluster, err := h.clusterCache.GetByIndex(cluster2.ByCluster, h.clusterName)
	if err != nil || len(cluster) != 1 {
		return configMap, err
	}

	fromConfigMap, err := configMapToSnapshots(configMap, configMap.Namespace, cluster[0].Name)
	if err != nil {
		return configMap, err
	}

	if !equality.Semantic.DeepEqual(cluster[0].Status.ETCDSnapshots, fromConfigMap) {
		cluster := cluster[0].DeepCopy()
		cluster.Status.ETCDSnapshots = fromConfigMap
		_, err = h.clusters.UpdateStatus(cluster)
		return configMap, err
	}

	return configMap, nil
}

func configMapToSnapshots(configMap *corev1.ConfigMap, clusterNamespace, clusterName string) ([]rkev1.ETCDSnapshot, error) {
	// sort
	return nil, nil
}
