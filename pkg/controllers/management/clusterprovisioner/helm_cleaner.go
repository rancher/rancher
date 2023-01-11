package clusterprovisioner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	apiMappings = &mapping.Metadata{
		Mappings: []*mapping.Mapping{
			{
				DeprecatedAPI:    "apiVersion: policy/v1beta1\nkind: PodSecurityPolicy\n",
				RemovedInVersion: "v1.25",
				// TODO add all APIs we want to cover here. Perhaps store it in a ConfigMap so we don't need to change
				// code for updates.
			},
		},
	}
)

type ClientGetter struct {
	k8sClient  kubernetes.Clientset
	restConfig rest.Config
}

func (c ClientGetter) ToRESTConfig() (*rest.Config, error) {
	return &c.restConfig, nil
}

func (c ClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return memory.NewMemCacheClient(c.k8sClient.Discovery()), nil
}

func (c ClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return meta.NewDefaultRESTMapper(nil), nil
}

func (c ClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return &clientcmd.DefaultClientConfig
}

func NewClientGetter(k8sClient kubernetes.Interface, restConfig rest.Config) ClientGetter {
	return ClientGetter{
		k8sClient:  *k8sClient.(*kubernetes.Clientset),
		restConfig: restConfig,
	}
}

func (p *Provisioner) cleanupHelmReleases(cluster *v3.Cluster) error {
	clusterManager := p.clusterManager
	client, err := clusterManager.K8sClient(cluster.Name)
	if err != nil {
		return errors.Wrapf(err, "[cleanupHelmReleases] failed to obtain the Kubernetes client instance")
	}

	restConfig, err := clusterManager.RESTConfig(cluster.Name)
	if err != nil {
		return errors.Wrapf(err, "[cleanupHelmReleases] failed to obtain the Kubernetes REST config instance")
	}

	listNamespaces, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "[cleanupHelmReleases] failed to list namespaces on cluster %v", cluster.Name)
	}

	clientGetter := NewClientGetter(client, *restConfig)
	helmDriver := os.Getenv("HELM_DRIVER")

	for _, namespace := range listNamespaces.Items {
		// TODO check if there's a way of doing this using a more API-based approach
		actionConfig := new(action.Configuration)

		if err = actionConfig.Init(
			clientGetter,
			namespace.Name,
			helmDriver,
			debugLog,
		); err != nil {
			return errors.Wrapf(err, "[cleanupHelmReleases] failed to create ActionConfiguration instance for Helm")
		}

		releases, err := actionConfig.Releases.ListReleases()
		if err != nil {
			return errors.Wrapf(err, "[cleanupHelmReleases] failed to list Helm releases for namespace %v", namespace.Name)
		}

		for _, helmRelease := range releases {
			lastRelease, err := actionConfig.Releases.Last(helmRelease.Name)
			if err != nil {
				logrus.Errorf("[cleanupHelmReleases] failed to find latest release version for release %v", helmRelease.Name)
				// If this fails, something went wrong. Skip to the next release.
				continue
			}

			// TODO consume the function from helm-mapkubeapis once that is merged in
			modifiedManifest, err := ReplaceManifestData(apiMappings, lastRelease.Manifest, cluster.Status.Version.GitVersion)
			if err != nil {
				// If this fails, it probably means we don't have adequate write permissions
				return errors.Wrapf(err, "[cleanupHelmReleases] failed to replace deprecated/removed APIs on cluster %v", cluster.Name)
			}

			if modifiedManifest == lastRelease.Manifest {
				logrus.Infof("[cleanupHelmReleases] release %v in namespace %v has no deprecated or removed APIs", lastRelease.Name, namespace.Name)
			} else if err := updateRelease(lastRelease, modifiedManifest, actionConfig); err != nil {
				logrus.Errorf("[cleanupHelmReleases] failed to update release %v in namespace %v, skipping...", lastRelease.Name, lastRelease.Namespace)
			}
		}
	}

	return nil
}

// debugLog is a function conforming to the DebugLog type in action.Configuration#Init to write debug messages
func debugLog(format string, v ...interface{}) {
	format = fmt.Sprintf("[cleanupHelmReleases][debug] %s\n", format)
	logrus.Debug(fmt.Sprintf(format, v...))
}

// ReplaceManifestData replaces the out-of-date APIs with their respective valid successors, or removes an API that
// does not have a successor.
// Logic extracted from https://github.com/stormqueen1990/helm-mapkubeapis/blob/0245b7a7837a36fd164d83e496c453811d62c083/pkg/common/common.go#L81-L142
func ReplaceManifestData(mapMetadata *mapping.Metadata, manifest string, kubeVersion string) (string, error) {
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
			return "", errors.Errorf("Failed to get the deprecated or removed Kubernetes version for API: %s", strings.ReplaceAll(deprecatedAPI, "\n", " "))
		}

		if count := strings.Count(manifest, deprecatedAPI); count > 0 {
			if semver.Compare(apiVersion, kubeVersion) > 0 {
				logrus.Printf("The following API does not require mapping as the "+
					"API is not deprecated or removed in Kubernetes '%s':\n\"%s\"\n", apiVersion,
					deprecatedAPI)
			} else {
				if supportedAPI == "" {
					logrus.Printf("Found %d instances of deprecated or removed Kubernetes API:\n\"%s\"\n", count, deprecatedAPI)

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
				} else {
					logrus.Printf("Found %d instances of deprecated or removed Kubernetes API:\n\"%s\"\nSupported API equivalent:\n\"%s\"\n", count, deprecatedAPI, supportedAPI)
					manifest = strings.ReplaceAll(manifest, deprecatedAPI, supportedAPI)
				}
			}
		}
	}
	return manifest, nil
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
