package autoscaler

import (
	"fmt"
	"reflect"

	"github.com/Masterminds/semver/v3"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	rke "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
					Version:     h.resolveHelmChartVersion(cluster),
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
		if !reflect.DeepEqual(bundle, helmOp.Spec) {
			helmOp = helmOp.DeepCopy()
			helmOp.Spec = bundle
			_, err = h.helmOp.Update(helmOp)
		}
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
func (h *autoscalerHandler) resolveHelmChartVersion(cluster *capi.Cluster) string {
	return chartVersions[h.getKubernetesMinorVersion(cluster)]
}

func (h *autoscalerHandler) getKubernetesMinorVersion(cluster *capi.Cluster) int {
	cp, err := h.dynamicClient.Get(
		cluster.Spec.ControlPlaneRef.GroupVersionKind(),
		cluster.Spec.ControlPlaneRef.Namespace,
		cluster.Spec.ControlPlaneRef.Name)
	if err != nil {
		logrus.Debugf("[autoscaler] no control-plane found for cluster %v/%v - latest version of cluster-autoscaler chart will be installed", cluster.Namespace, cluster.Name)
		return 0
	}

	k8sVersionStr := ""

	// handle v2prov not adhering to capi for the `Version` field
	apiVersion, _ := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if apiVersion == "rke.cattle.io/v1" {
		obj, ok := cp.(*rke.RKEControlPlane)
		if !ok {
			return 0
		}
		k8sVersionStr = obj.Spec.KubernetesVersion
	} else {
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cp)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to convert object to unstructured for cluster %v/%v - latest version of cluster-autoscaler chart will be installed", cluster.Namespace, cluster.Name)
			return 0
		}

		v, ok, err := unstructured.NestedFieldNoCopy(obj, "spec", "version")
		if !ok || err != nil {
			logrus.Debugf("[autoscaler] failed to get CAPI version field from unstructured object for cluster %v/%v: ok=%v, err=%v", cluster.Namespace, cluster.Name, ok, err)
			return 0
		}
		k8sVersionStr, ok = v.(string)
		if !ok {
			logrus.Debugf("[autoscaler] failed to convert version field to string for cluster %v/%v: type assertion failed", cluster.Namespace, cluster.Name)
			return 0
		}
	}

	version, err := semver.NewVersion(k8sVersionStr)
	if err != nil {
		logrus.Debugf("[autoscaler] failed to parse kubernetes version '%s' for cluster %v/%v: %v", k8sVersionStr, cluster.Namespace, cluster.Name, err)
		return 0
	}

	return int(version.Minor())
}
