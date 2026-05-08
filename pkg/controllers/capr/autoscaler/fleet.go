package autoscaler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/docker/distribution/reference"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/external"
)

type k8sToAutoscalerVersion struct {
	imageTag     string
	chartVersion string
}

// hardcoded k8s minor <-> imageTag tag + chartVersion version mapping, adding new versions here will automatically
// rollout updates to all clusters on rancher upgrade (e.g. setting a new minor version for imageTag or chartVersion)
var k8sVersionToAutoscalerChartVersions = map[int]*k8sToAutoscalerVersion{
	35: {
		imageTag:     "1.35.0-4.1",
		chartVersion: "9.56.0",
	},
	34: {
		imageTag:     "1.34.0-3.4",
		chartVersion: "9.50.1",
	},
	33: {
		imageTag:     "1.33.0-3.3",
		chartVersion: "9.50.1",
	},
	32: {
		imageTag:     "1.32.3-1.5",
		chartVersion: "9.50.1",
	},
}

// this is a default value - so we never actually fall back to what is in the chart. this ensures
// that we're running a vetted image that exists in the prime registry
var defaultChartVersionConfigs = k8sVersionToAutoscalerChartVersions[34]

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
					Chart:       getChartName(),
					Version:     h.chartVersionsForCluster(cluster).chartVersion,
					Repo:        settings.ClusterAutoscalerChartRepository.Get(),
					ReleaseName: "cluster-autoscaler",
					Values: &fleet.GenericMap{
						Data: map[string]any{
							"replicaCount": replicaCount,
							"image":        h.getChartImageSettings(cluster),
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

func (h *autoscalerHandler) chartVersionsForCluster(cluster *capi.Cluster) *k8sToAutoscalerVersion {
	v := h.getKubernetesMinorVersion(cluster)
	versions, ok := k8sVersionToAutoscalerChartVersions[v]
	if !ok {
		logrus.Warnf(
			"[autoscaler] no chart versions found for cluster %s/%s with kubernetes minor=%d, using default chartVersion=%s imageTag=%s",
			cluster.Namespace,
			cluster.Name,
			v,
			defaultChartVersionConfigs.chartVersion,
			defaultChartVersionConfigs.imageTag,
		)
		return defaultChartVersionConfigs
	}

	return versions
}

// getChartImageSettings returns a map of the image settings to pass to the chart, this is based on the kubernetes minor version
func (h *autoscalerHandler) getChartImageSettings(cluster *capi.Cluster) map[string]any {
	// if we don't specify an image - just use whatever is in the chart
	autoscalerImage := settings.ClusterAutoscalerImage.Get()
	if autoscalerImage == "" {
		return map[string]any{}
	}

	// parse out the image to properly set all the values in the chart
	imageRef, err := reference.ParseNamed(autoscalerImage)
	if err != nil {
		logrus.Debugf("[autoscaler] failed to parse autoscaler image '%s': %v", autoscalerImage, err)
		return map[string]any{}
	}

	registry := reference.Domain(imageRef)
	image := reference.Path(imageRef)
	tag, isTagged := imageRef.(reference.NamedTagged)

	// if we are not overriding all the image settings fall back to whatever is in the chart by default
	if registry == "" && image == "" {
		return map[string]any{}
	}

	imageSettings := map[string]any{
		"repository": image,
		"registry":   registry,
	}

	// this handles if the image setting was set with a tag - we just use that
	// instead of the hardcoded version for the k8s version
	if isTagged {
		imageSettings["tag"] = tag.Tag()
	} else {
		imageSettings["tag"] = h.chartVersionsForCluster(cluster).imageTag
	}

	return imageSettings
}

// getChartName returns the cluster-autoscaler chart name based on the chart repository URL prefix. OCI charts do not need a chart-name as it is referring
// to an OCI image.
func getChartName() string {
	if strings.HasPrefix(settings.ClusterAutoscalerChartRepository.Get(), "oci://") {
		return ""
	}

	return "cluster-autoscaler"
}

// getKubernetesMinorVersion returns the k8s minor version which is looked up from the controlPlaneRef on the capi object
func (h *autoscalerHandler) getKubernetesMinorVersion(cluster *capi.Cluster) int {
	if !cluster.Spec.ControlPlaneRef.IsDefined() {
		logrus.Debugf("[autoscaler] no control-plane ref found for cluster %s/%s - latest version of cluster-autoscaler chart will be installed", cluster.Namespace, cluster.Name)
		return 0
	}

	// Use CAPI's external package to get the control plane object with automatic version discovery
	cp, err := external.GetObjectFromContractVersionedRef(h.context, h.client, cluster.Spec.ControlPlaneRef, cluster.Namespace)
	if err != nil {
		logrus.Debugf("[autoscaler] failed to get control-plane for cluster %s/%s: %v - latest version of cluster-autoscaler chart will be installed", cluster.Namespace, cluster.Name, err)
		return 0
	}

	k8sVersionStr := ""

	// handle v2prov not adhering to capi for the `Version` field
	cpAPIVersion, _ := cp.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if cpAPIVersion == "rke.cattle.io/v1" {
		// For RKE control planes, the kubernetes version is in spec.kubernetesVersion
		v, ok, err := unstructured.NestedString(cp.Object, "spec", "kubernetesVersion")
		if !ok || err != nil {
			logrus.Debugf("[autoscaler] failed to get kubernetesVersion field from RKE control plane for cluster %s/%s: ok=%v, err=%v", cluster.Namespace, cluster.Name, ok, err)
			return 0
		}
		k8sVersionStr = v
	} else {
		// For CAPI control planes, the kubernetes version is in spec.version
		v, ok, err := unstructured.NestedString(cp.Object, "spec", "version")
		if !ok || err != nil {
			logrus.Debugf("[autoscaler] failed to get CAPI version field from unstructured object for cluster %s/%s: ok=%v, err=%v", cluster.Namespace, cluster.Name, ok, err)
			return 0
		}
		k8sVersionStr = v
	}

	version, err := semver.NewVersion(k8sVersionStr)
	if err != nil {
		logrus.Debugf("[autoscaler] failed to parse kubernetes version '%s' for cluster %s/%s: %v", k8sVersionStr, cluster.Namespace, cluster.Name, err)
		return 0
	}

	return int(version.Minor())
}

// cleanupFleet removes all fleet-related resources for a given cluster
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
