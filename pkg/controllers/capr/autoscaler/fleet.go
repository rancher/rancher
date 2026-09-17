package autoscaler

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/docker/distribution/reference"
	fleet "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/capr"
	provimage "github.com/rancher/rancher/pkg/provisioningv2/image"
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
	helmOpSecretName, imagePullSecretName, err := h.manageHelmOpSecrets(cluster)
	if err != nil {
		return err
	}
	repository := h.getChartRepository(cluster)

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
					Chart:       getChartName(repository),
					Version:     h.chartVersionsForCluster(cluster).chartVersion,
					Repo:        repository,
					ReleaseName: "cluster-autoscaler",
					Values: &fleet.GenericMap{
						Data: map[string]any{
							"replicaCount": replicaCount,
							"image":        h.getChartImageSettings(cluster, imagePullSecretName),
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

	if helmOpSecretName != "" {
		bundle.DownstreamResources = append(bundle.DownstreamResources, fleet.DownstreamResource{
			Kind: "secret",
			Name: helmOpSecretName,
		})
		bundle.HelmSecretName = helmOpSecretName
		bundle.HelmOpOptions = &fleet.BundleHelmOptions{
			SecretName: helmOpSecretName,
		}
	}

	if imagePullSecretName != "" {
		bundle.DownstreamResources = append(bundle.DownstreamResources, fleet.DownstreamResource{
			Kind: "secret",
			Name: imagePullSecretName,
		})
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
func (h *autoscalerHandler) getChartImageSettings(cluster *capi.Cluster, pullSecretName string) map[string]any {
	imageSettings := map[string]any{}
	if pullSecretName != "" {
		imageSettings["pullSecrets"] = []string{pullSecretName}
	}

	var autoscalerImage string

	autoscalerOverrideImage := settings.ClusterAutoscalerImage.Get()
	// if the setting _isn't_ set, we resolve the image based on the provisioning cluster's registry configuration
	if autoscalerOverrideImage == "" {
		var provCluster *provv1.Cluster
		if cluster != nil {
			var err error
			provCluster, err = capr.GetProvisioningClusterFromCAPICluster(cluster, h.clusterCache)
			if err != nil {
				logrus.Debugf("[autoscaler] failed to find provisioning cluster for autoscaler image: %v", err)
				return imageSettings
			}
		}
		autoscalerImage = provimage.ResolveWithCluster("rancher/appco-kubernetes-cluster-autoscaler", provCluster)
	} else {
		// otherwise, we just use the overridden image.
		autoscalerImage = autoscalerOverrideImage
	}

	// parse out the image to properly set all the values in the chart (resolved from the cluster or from the setting)
	imageRef, err := reference.ParseNormalizedNamed(autoscalerImage)
	if err != nil {
		logrus.Debugf("[autoscaler] failed to parse autoscaler image '%s': %v", autoscalerImage, err)
		return imageSettings
	}

	registry := reference.Domain(imageRef)
	image := reference.Path(imageRef)
	tag, isTagged := imageRef.(reference.NamedTagged)

	// if we are not overriding all the image settings fall back to whatever is in the chart by default
	if registry == "" && image == "" {
		return imageSettings
	}

	imageSettings["repository"] = image
	imageSettings["registry"] = registry

	// this handles if the image setting was set with a tag - we just use that
	// instead of the hardcoded version for the k8s version
	if isTagged {
		imageSettings["tag"] = tag.Tag()
	} else {
		imageSettings["tag"] = h.chartVersionsForCluster(cluster).imageTag
	}

	return imageSettings
}

// getChartName returns the chart name required by an HTTPS Helm repository.
// OCI repositories include the chart name in their repository path.
func getChartName(repository string) string {
	if strings.HasPrefix(repository, "oci://") {
		return ""
	}

	return "cluster-autoscaler"
}

// getChartRepository returns the chart repository URL to use for the Fleet HelmOp.
// An explicit chart repository setting is preserved. Otherwise the default autoscaler
// image is resolved against the cluster registry and represented as an OCI URL.
func (h *autoscalerHandler) getChartRepository(cluster *capi.Cluster) string {
	repo := settings.ClusterAutoscalerChartRepository.Get()
	if repo != "" {
		return repo
	}

	var provCluster *provv1.Cluster
	if cluster != nil {
		var err error
		provCluster, err = capr.GetProvisioningClusterFromCAPICluster(cluster, h.clusterCache)
		if err != nil {
			logrus.Debugf("[autoscaler] failed to find provisioning cluster for autoscaler repository: %v", err)
			return ""
		}
	}

	// this falls back to the global private registry if provCluster is still nil
	registry, _ := provimage.GetPrivateRepoURLFromCluster(provCluster)
	if registry == "" {
		logrus.Warnf("[autoscaler] no available private registry found for cluster %s/%s - unable to resolve chart repository for cluster-autoscaler", cluster.Namespace, cluster.Name)
		return ""
	}

	return "oci://" + provimage.ResolveWithCluster("rancher/charts/appco-kubernetes-cluster-autoscaler", provCluster)
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

	// Delete the cluster scoped autoscaler secrets if they exist
	provCluster, err := h.clusterCache.Get(cluster.Namespace, cluster.Name)
	if err == nil {
		err = h.cleanupClusterScopedSecrets(provCluster, cluster)
		if err != nil {
			errs = append(errs, err)
		}
	} else if !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to check existence of Cluster %s in namespace %s when cleaning up Helm operation secrets: %w", cluster.Name, cluster.Namespace, err))
	}

	// Return combined errors if any occurred
	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors during fleet cleanup: %v", len(errs), errs)
	}

	return nil
}
