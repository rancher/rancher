package autoscaler

import (
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/settings"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"
)

// hardcoded k8s minor <-> chart version mapping, adding new versions here will automatically
// rollout updates to all clusters on rancher upgrade (e.g. setting a new minor version)
var chartVersions = map[int]string{
	33: "9.50.1",
	32: "9.46.6",
	31: "9.44.0",
}

// ensureFleetHelmOp creates or updates a Helm operation for cluster autoscaler.
// one key parameter here is the kubeconfigVersion which is legitimately just involved to
// force a re-rollout of the downstream cluster-autoscaler deployment on token-rotation.
func (h *autoscalerHandler) ensureFleetHelmOp(cluster *capi.Cluster, kubeconfigVersion string, k8sMinorVersion, replicaCount int) error {
	bundle := fleet.HelmOpSpec{
		BundleSpec: fleet.BundleSpec{
			Targets: []fleet.BundleTarget{
				{
					ClusterName: cluster.Name,
				},
			},
			BundleDeploymentOptions: fleet.BundleDeploymentOptions{
				DefaultNamespace: "kube-system",
				Helm: &fleet.HelmOptions{
					Chart:       "cluster-autoscaler",
					Version:     chartVersions[k8sMinorVersion],
					Repo:        settings.ClusterAutoscalerChartRepo.Get(),
					ReleaseName: "cluster-autoscaler",
					Values: &fleet.GenericMap{
						Data: map[string]any{
							"replicaCount": replicaCount,
							"autoDiscovery": map[string]any{
								"clusterName": cluster.Name,
								"namespace":   cluster.Namespace,
							},
							"cloudProvider":             "clusterapi",
							"clusterAPIMode":            "incluster-kubeconfig",
							"clusterAPICloudConfigPath": "/etc/kubernetes/mgmt-cluster/value",
							"extraVolumeSecrets": map[string]any{
								"local-cluster": map[string]any{
									"name":      "mgmt-kubeconfig",
									"mountPath": "/etc/kubernetes/mgmt-cluster",
								},
							},
							"extraArgs": map[string]any{
								"v": 2,
							},
							"extraEnv": map[string]any{
								// not necessary for functionality - only needed for lifecycle tracking
								// e.g. new rollout whenever kubeconfig updates.
								"RANCHER_AUTOSCALER_KUBECONFIG_VERSION": kubeconfigVersion,
							},
						},
					},
				},
			},
		},
	}

	helmOp, err := h.helmOpCache.Get(cluster.Namespace, helmOpName(cluster))
	if errors.IsNotFound(err) {
		_, err = h.helmOp.Create(&fleet.HelmOp{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       cluster.Namespace,
				Name:            helmOpName(cluster),
				OwnerReferences: ownerReference(cluster),
				Labels: map[string]string{
					capi.ClusterNameLabel: cluster.Name,
				},
			},
			Spec: bundle,
		})
		return err
	} else if err == nil {
		helmOp = helmOp.DeepCopy()
		helmOp.Spec = bundle
		_, err = h.helmOp.Update(helmOp)
		return err
	}

	return err
}

func (h *autoscalerHandler) uninstallHelmOp(cluster *capi.Cluster) error {
	helmOp, err := h.helmOpCache.Get(cluster.Namespace, helmOpName(cluster))
	if err != nil {
		return err
	}

	return h.helmOp.Delete(helmOp.Namespace, helmOp.Name, &metav1.DeleteOptions{})
}
