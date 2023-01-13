package clusterprovisioner

import (
	"strings"

	"github.com/rancher/rancher/pkg/data/util"

	"github.com/rancher/rancher/pkg/wrangler"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/helm/helm-mapkubeapis/pkg/mapping"
	"github.com/pkg/errors"
	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/mod/semver"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

var (
	apiMappings = &mapping.Metadata{
		Mappings: []*mapping.Mapping{
			{
				DeprecatedAPI:    "apiVersion: policy/v1beta1\nkind: PodSecurityPolicy\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: PodSecurityPolicy\napiVersion: policy/v1beta1\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: policy/v1beta1\r\nkind: PodSecurityPolicy\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: PodSecurityPolicy\r\napiVersion: policy/v1beta1\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: batch/v1beta1\nkind: CronJob\n",
				NewAPI:           "apiVersion: batch/v1\nkind: CronJob\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: CronJob\napiVersion: batch/v1beta1\n",
				NewAPI:           "apiVersion: batch/v1\nkind: CronJob\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: batch/v1beta1\r\nkind: CronJob\r\n",
				NewAPI:           "apiVersion: batch/v1\r\nkind: CronJob\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: CronJob\r\napiVersion: batch/v1beta1\r\n",
				NewAPI:           "apiVersion: batch/v1\r\nkind: CronJob\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: autoscaling/v2beta1\nkind: HorizontalPodAutoscaler\n",
				NewAPI:           "apiVersion: autoscaling/v2\nkind: HorizontalPodAutoscaler\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: HorizontalPodAutoscaler\napiVersion: autoscaling/v2beta1\n",
				NewAPI:           "apiVersion: autoscaling/v2\nkind: HorizontalPodAutoscaler\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: autoscaling/v2beta1\r\nkind: HorizontalPodAutoscaler\r\n",
				NewAPI:           "apiVersion: autoscaling/v2\r\nkind: HorizontalPodAutoscaler\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: HorizontalPodAutoscaler\r\napiVersion: autoscaling/v2beta1\r\n",
				NewAPI:           "apiVersion: autoscaling/v2\r\nkind: HorizontalPodAutoscaler\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: policy/v1beta1\nkind: PodDisruptionBudget\n",
				NewAPI:           "apiVersion: policy/v1\nkind: PodDisruptionBudget\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: PodDisruptionBudget\napiVersion: policy/v1beta1\n",
				NewAPI:           "apiVersion: policy/v1\nkind: PodDisruptionBudget\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "apiVersion: policy/v1beta1\r\nkind: PodDisruptionBudget\r\n",
				NewAPI:           "apiVersion: policy/v1\r\nkind: PodDisruptionBudget\r\n",
				RemovedInVersion: "v1.25",
			},
			{
				DeprecatedAPI:    "kind: PodDisruptionBudget\r\napiVersion: policy/v1beta1\r\n",
				NewAPI:           "apiVersion: policy/v1\r\nkind: PodDisruptionBudget\r\n",
				RemovedInVersion: "v1.25",
			},
		},
	}

	// FeatureAppNS is a list of feature namespaces to clean up Helm releases from.
	FeatureAppNS = util.FeatureAppNS
)

// EmptyHelmDriverName is a placeholder for the empty Helm driver.
const EmptyHelmDriverName = ""

func newClientGetter(k8sClient kubernetes.Interface, restConfig rest.Config) *wrangler.SimpleRESTClientGetter {
	return &wrangler.SimpleRESTClientGetter{
		ClientConfig:    &clientcmd.DefaultClientConfig,
		RESTConfig:      &restConfig,
		CachedDiscovery: memory.NewMemCacheClient(k8sClient.Discovery()),
		RESTMapper:      meta.NewDefaultRESTMapper(nil),
	}
}

func (p *Provisioner) cleanupHelmReleases(cluster *v3.Cluster) error {
	clusterManager := p.clusterManager
	userContext, err := clusterManager.UserContextNoControllers(cluster.Name)
	if err != nil {
		return errors.Wrapf(err, "[cleanupHelmReleases] failed to obtain the Kubernetes client instance")
	}

	clientGetter := newClientGetter(userContext.K8sClient, userContext.RESTConfig)

	for _, namespace := range FeatureAppNS {
		actionConfig := &action.Configuration{}
		if err = actionConfig.Init(clientGetter, namespace, EmptyHelmDriverName, logrus.Debugf); err != nil {
			return errors.Wrapf(err, "[cleanupHelmReleases] failed to create ActionConfiguration instance for Helm")
		}

		listAction := action.NewList(actionConfig)
		releases, err := listAction.Run()
		if err != nil {
			return errors.Wrapf(err, "[cleanupHelmReleases] failed to list Helm releases for namespace %v", namespace)
		}

		for _, helmRelease := range releases {
			lastRelease, err := actionConfig.Releases.Last(helmRelease.Name)
			if err != nil {
				logrus.Errorf("[cleanupHelmReleases] failed to find latest release version for release %v", helmRelease.Name)
				// If this fails, something went wrong. Skip to the next release.
				continue
			}

			// TODO consume the function from helm-mapkubeapis once that is merged in
			replaced, modifiedManifest, err := ReplaceManifestData(apiMappings, lastRelease.Manifest, cluster.Status.Version.GitVersion)
			if err != nil {
				// If this fails, it probably means we don't have adequate write permissions
				return errors.Wrapf(err, "[cleanupHelmReleases] failed to replace deprecated/removed APIs on cluster %v", cluster.Name)
			}

			if !replaced {
				logrus.Infof("[cleanupHelmReleases] release %v in namespace %v has no deprecated or removed APIs", lastRelease.Name, namespace)
				continue
			}

			if err := updateRelease(lastRelease, modifiedManifest, actionConfig); err != nil {
				logrus.Errorf("[cleanupHelmReleases] failed to update release %v in namespace %v, skipping...", lastRelease.Name, lastRelease.Namespace)
			}
		}
	}

	return nil
}

// ReplaceManifestData replaces the out-of-date APIs with their respective valid successors, or removes an API that
// does not have a successor.
// Logic extracted from https://github.com/stormqueen1990/helm-mapkubeapis/blob/0245b7a7837a36fd164d83e496c453811d62c083/pkg/common/common.go#L81-L142
func ReplaceManifestData(mapMetadata *mapping.Metadata, manifest string, kubeVersion string) (bool, string, error) {
	if !semver.IsValid(kubeVersion) {
		return false, "", errors.Errorf("Invalid format for Kubernetes semantic version: %v", kubeVersion)
	}

	var replaced = false
	for _, mappingData := range mapMetadata.Mappings {
		deprecatedAPI := mappingData.DeprecatedAPI
		supportedAPI := mappingData.NewAPI
		var apiVersion string

		if mappingData.DeprecatedInVersion != "" {
			apiVersion = mappingData.DeprecatedInVersion
		} else {
			apiVersion = mappingData.RemovedInVersion
		}

		if !semver.IsValid(apiVersion) {
			return replaced, "", errors.Errorf("Failed to get the deprecated or removed Kubernetes version for API: %s", strings.ReplaceAll(deprecatedAPI, "\n", " "))
		}

		var count int
		if count = strings.Count(manifest, deprecatedAPI); count <= 0 {
			continue
		}

		if semver.Compare(apiVersion, kubeVersion) > 0 {
			logrus.Debugf("The following API does not require mapping as the "+
				"API is not deprecated or removed in Kubernetes '%s':\n\"%s\"\n", apiVersion,
				deprecatedAPI)
			continue
		}

		if supportedAPI == "" {
			logrus.Debugf("Found %d instances of deprecated or removed Kubernetes API:\n\"%s\"\n", count, deprecatedAPI)
			manifest = removeResourceWithNoSuccessors(count, manifest, deprecatedAPI)
		} else {
			logrus.Debugf("Found %d instances of deprecated or removed Kubernetes API:\n\"%s\"\nSupported API equivalent:\n\"%s\"\n", count, deprecatedAPI, supportedAPI)
			manifest = strings.ReplaceAll(manifest, deprecatedAPI, supportedAPI)
		}

		replaced = true
	}

	return replaced, manifest, nil
}

// removeResourceWithNoSuccessors removes a resource for which its respective API has no successors.
func removeResourceWithNoSuccessors(count int, manifest string, deprecatedAPI string) string {
	for repl := 0; repl < count; repl++ {
		// find the position where the API header is
		apiIndex := strings.Index(manifest, deprecatedAPI)

		// find the next separator index
		separatorIndex := strings.Index(manifest[apiIndex:], "---\n")

		// find the previous separator index
		previousSeparatorIndex := strings.LastIndex(manifest[:apiIndex], "---\n")

		/*
		 * if no previous separator index was found, it means the resource is at the beginning and not
		 * prefixed by ---
		 */
		if previousSeparatorIndex == -1 {
			if apiIndex == 0 {
				previousSeparatorIndex = 0
			} else {
				previousSeparatorIndex = apiIndex - 1
			}
		}

		if separatorIndex == -1 { // this means we reached the end of input
			manifest = manifest[:previousSeparatorIndex]
		} else {
			manifest = manifest[:previousSeparatorIndex] + manifest[separatorIndex+apiIndex:]
		}
	}

	manifest = strings.Trim(manifest, "\n")
	return manifest
}

// updateRelease updates a release in the cluster with an equivalent with the superseded APIs replaced or removed as
// needed.
// Logic extracted from https://github.com/helm/helm-mapkubeapis/blob/main/pkg/v3/release.go#L71-L94
func updateRelease(originalRelease *release.Release, modifiedManifest string, config *action.Configuration) error {
	originalRelease.Info.Status = release.StatusSuperseded
	if err := config.Releases.Update(originalRelease); err != nil {
		return errors.Wrapf(err, "[updateRelease] failed to update original release %v in namespace %v", originalRelease.Name, originalRelease.Namespace)
	}

	newRelease := originalRelease
	newRelease.Manifest = modifiedManifest
	newRelease.Info.Description = UpgradeDescription
	newRelease.Info.LastDeployed = config.Now()
	newRelease.Version = originalRelease.Version + 1
	newRelease.Info.Status = release.StatusDeployed

	logrus.Infof("[updateRelease] add release version %v for release %v with updated supported APIs in namespace %v", originalRelease.Version, originalRelease.Name, originalRelease.Namespace)

	if err := config.Releases.Create(newRelease); err != nil {
		return errors.Wrapf(err, "[updateRelease] failed to create new release version %v for release %v in namespace %v", newRelease.Version, newRelease.Name, newRelease.Namespace)
	}

	logrus.Infof("[updateRelease] successfully created new version for release %v in namespace %v", newRelease.Name, newRelease.Namespace)

	return nil
}
