package autoscaler

import (
	"fmt"

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
func (h *autoscalerHandler) ensureFleetHelmOp(cluster *capi.Cluster, kubeconfigVersion string, replicaCount int) error {
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
					Version:     helmChartVersionForCapiCluster(cluster),
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

// cleanup removes all fleet-related resources for a given cluster
func (h *autoscalerHandler) cleanupFleet(cluster *capi.Cluster) error {
	var errs []error

	// Delete the Helm operation if it exists
	helmOpName := helmOpName(cluster)
	if _, err := h.helmOpCache.Get(cluster.Namespace, helmOpName); err == nil {
		if err := h.helmOp.Delete(cluster.Namespace, helmOpName, &metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete Helm operation %s in namespace %s: %w", helmOpName, cluster.Namespace, err))
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of Helm operation %s in namespace %s: %w", helmOpName, cluster.Namespace, err))
	}

	// Return combined errors if any occurred
	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors during fleet cleanup: %v", len(errs), errs)
	}

	return nil
}

// Returns the Helm chart version for cluster autoscaler based on the Kubernetes minor version of the cluster.
func helmChartVersionForCapiCluster(cluster *capi.Cluster) string {
	return chartVersions[k8sMinorVersion(cluster)]
}
