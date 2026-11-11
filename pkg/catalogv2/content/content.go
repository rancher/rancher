/*
Package content provides a set of tools for manipulating Helm charts and Helm repositories, as well as managing their dependencies.

It includes:
  - Objects and methods for interacting with Helm charts and repositories.
  - Auto-generated code by Wrangler, encapsulated in high-level functions.
  - Structs for representing Helm chart repositories and caching Helm chart indexes.
  - Methods for creating a new instance of a Helm repository manager, fetching Helm repository information, retrieving Helm chart icons, retrieving Helm charts, and extracting detailed information about a Helm chart.

The package is primarily used to create an index file of a Helm repository (represented as a ClusterRepo custom resource), retrieve chart information and chart icons from the index file, and provide these functionalities with thread safety.

It includes a Manager struct that provides a set of functionalities to interact with Helm repositories. It uses clientsets provided by the Wrangler library to interact with the Kubernetes API and quickly fetch instances of ConfigMaps, Secrets, and ClusterRepos.

The package is designed to be used by developers who are working with Helm charts in a Kubernetes environment and who need to interact with Helm repositories, retrieve chart information and chart icons, and manage Helm chart dependencies.
*/
package content

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	"github.com/rancher/rancher/pkg/catalogv2/helm"
	helmhttp "github.com/rancher/rancher/pkg/catalogv2/http"
	"github.com/rancher/rancher/pkg/catalogv2/internal/cache"
	"github.com/rancher/rancher/pkg/catalogv2/oci"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
	"helm.sh/helm/v4/pkg/registry"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

// Manager is a struct that provides a set of functionalities to interact with Helm repositories.
// It is primarily used to create an index file of a Helm repository (represented as a ClusterRepo custom resource),
// retrieve chart information and chart icons from the index file.
//
// The Manager struct uses clientsets provided by the wrangler library
// to interact with the Kubernetes API and quickly fetch instances of ConfigMaps, Secrets, and ClusterRepos.
type Manager struct {
	configMaps   corecontrollers.ConfigMapCache      // clientset cache for ConfigMaps.
	secrets      corecontrollers.SecretCache         // clientset cache for Secrets.
	clusterRepos catalogcontrollers.ClusterRepoCache // clientset cache for ClusterRepo custom resources.
	discovery    discovery.DiscoveryInterface        // An interface to the Kubernetes Discovery API. Provides information about the Kubernetes API server.
	IndexCache   map[string]indexCache               // cache for Helm repository index files. Used to store and retrieve index files for faster access.
	lock         sync.RWMutex                        // read-write mutex used to ensure that some Manager's operations are thread-safe.
	chartCache   cache.CAS                           // Content-addressable storage cache for chart archives keyed by sha256 digest.
	sf           singleflight.Group                  // Single-flight group to deduplicate concurrent remote chart fetches.
}

// indexCache - used to cache helm chart indexes
type indexCache struct {
	index    *repo.IndexFile // Pointer to the helm chart index
	revision string          // The revision number of the index in the Kubernetes API server
}

// repoDef is used to represent a Helm chart repository.
type repoDef struct {
	typedata *metav1.TypeMeta   // Metadata that describes the API version and kind of the Kubernetes custom resource that defines the repository
	metadata *metav1.ObjectMeta // Metadata that describes the name, namespace, and other attributes of the repository
	spec     *v1.RepoSpec       // The specification of the repository, including its URL, authentication credentials, and other settings
	status   *v1.RepoStatus     // The current status of the repository, including its commit SHA and the name of the ConfigMap that holds its index
}

// NewManager creates a new pointer for Manager struct
func NewManager(
	discovery discovery.DiscoveryInterface,
	configMaps corecontrollers.ConfigMapCache,
	secrets corecontrollers.SecretCache,
	clusterRepos catalogcontrollers.ClusterRepoCache) (*Manager, error) {
	chartCache, err := cache.NewDiskCAS("")
	if err != nil {
		return nil, err
	}
	return &Manager{
		discovery:    discovery,
		configMaps:   configMaps,
		secrets:      secrets,
		clusterRepos: clusterRepos,
		IndexCache:   map[string]indexCache{},
		chartCache:   chartCache,
	}, nil
}

// SetChartCache overrides the CAS chart cache implementation (primarily used in tests).
func (c *Manager) SetChartCache(chartCache cache.CAS) {
	c.chartCache = chartCache
}

// Index (thread-safe) retrieves the Helm repository information for a specific namespace and name.
// By default, it uses rancher version and the local cluster's k8s version to filter available versions in the returned index file;
// If skipFilter is true, it will return the entire unfiltered index file;
// if a valid targetK8sVersion is provided, it will filter versions based on rancher version and the target k8s version.
func (c *Manager) Index(namespace, name, targetK8sVersion string, skipFilter bool) (*repo.IndexFile, error) {
	r, err := c.getRepo(namespace, name)
	if err != nil {
		return nil, err
	}

	cm, err := c.configMaps.Get(r.status.IndexConfigMapNamespace, r.status.IndexConfigMapName)
	if err != nil {
		return nil, err
	}
	var k8sVersion *semver.Version
	if targetK8sVersion != "" {
		k8sVersion, err = semver.NewVersion(targetK8sVersion)
		if err != nil {
			return nil, err
		}
	} else {
		k8sVersion, err = c.k8sVersion()
		if err != nil {
			return nil, err
		}
	}
	// Check IndexCache and if it is up-to-date.
	c.lock.RLock()
	if cache, ok := c.IndexCache[fmt.Sprintf("%s/%s", r.status.IndexConfigMapNamespace, r.status.IndexConfigMapName)]; ok {
		if cm.ResourceVersion == cache.revision {
			c.lock.RUnlock()
			return c.filterReleases(deepCopyIndex(cache.index), k8sVersion, skipFilter), nil
		}
	}
	c.lock.RUnlock()

	if len(cm.OwnerReferences) == 0 || cm.OwnerReferences[0].UID != r.metadata.UID {
		return nil, validation.Unauthorized
	}

	data, err := c.readBytes(cm)
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	data, err = io.ReadAll(gz)
	if err != nil {
		return nil, err
	}

	// Unmarshall the fetched ConfigMap since the Index cache is not up-to-date
	index := &repo.IndexFile{}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, err
	}

	c.lock.Lock()
	c.IndexCache[fmt.Sprintf("%s/%s", r.status.IndexConfigMapNamespace, r.status.IndexConfigMapName)] = indexCache{
		index:    index,
		revision: cm.ResourceVersion,
	}
	c.lock.Unlock()

	return c.filterReleases(deepCopyIndex(index), k8sVersion, skipFilter), nil
}

// Icon Returns an io.ReadCloser and the icon's MIME type for the chart.
//
// If the chart's icon is not an HTTP or HTTPS URL, retrieves the icon from the repo's Git repository.
// Otherwise, retrieves the icon via HTTP from the chart's URL and returns it as an io.ReadCloser with the proper Secret.
func (c *Manager) Icon(namespace, name, chartName, version string) (io.ReadCloser, string, error) {
	index, err := c.Index(namespace, name, "", true)
	if err != nil {
		return nil, "", err
	}

	chart, err := index.Get(chartName, version)
	if err != nil {
		return nil, "", err
	}

	repo, err := c.getRepo(namespace, name)
	if err != nil {
		return nil, "", err
	}

	// If the chart icon is not an HTTP URL and the repository has a commit status,
	// attempt to get the icon from the git repository.
	if !isHTTP(chart.Icon) && repo.status.Commit != "" {
		icon, suffix, err := git.Icon(namespace, name, repo.status.URL, chart)
		return icon, suffix, notReady(err)
	}

	// Check if the repository from the chart is bundled and is at an airgapped environment
	rancherBundled := isRancherAndBundledCatalog(repo)
	if rancherBundled {
		// If the icon is not available in the git repository, use the fallback icon for airgapped environments.
		// which will be handled by the UI, as long as this returns a nil io.ReadCloser and nil error.
		return nil, "", nil
	}

	secret, err := catalogv2.GetSecret(c.secrets, repo.spec, repo.metadata.Namespace)
	if err != nil {
		return nil, "", err
	}

	return helmhttp.Icon(secret, repo.status.URL, repo.spec.CABundle, repo.spec.InsecureSkipTLSverify, repo.spec.DisableSameOriginCheck, chart)
}

// Chart retrieves a specific Helm chart from a Helm repository.
//
// Retrieves the index file, fetches the helm chart and repository data.
// Check's the commit status of the repository
//
// If the commit status of the repository is not an empty string,
// Return the Chart through Git without secret
//
// If the commit status of the repository is an empty string,
// it retrieves the secret associated with the repository
//
// The function returns an io.ReadCloser which represents the chart content.
func (c *Manager) Chart(namespace, name, chartName, version string, skipFilter bool) (io.ReadCloser, error) {
	index, err := c.Index(namespace, name, "", skipFilter)
	if err != nil {
		return nil, err
	}

	chart, err := index.Get(chartName, version)
	if err != nil {
		return nil, err
	} else if len(chart.URLs) == 0 {
		return nil, fmt.Errorf("failed to find chartName %s version %s: %w", chart.Name, chart.Version, validation.NotFound)
	}
	chartURL := chart.URLs[0]
	digest := chart.Digest

	// CAS disk cache hit: If this chart archive was previously downloaded and verified, serve it directly.
	canUseCache := digest != "" && c.chartCache != nil
	if canUseCache {
		if cached, err := c.chartCache.Get(digest); err == nil {
			return cached, nil
		}
	}

	// Retrieve the clusterRepo
	repo, err := c.getRepo(namespace, name)
	if err != nil {
		return nil, err
	}

	// Git-based ClusterRepos load local charts from the checkout. Remote charts (OCI or HTTP)
	// are pulled from their respective remote locations, with a local-first check for bundled assets.
	var secret *corev1.Secret
	if repo.status.Commit != "" {
		// Classic git-based ClusterRepo, charts served from the checkout.
		if isLocalChart(chartURL, repo.status.URL) {
			return git.Chart(namespace, name, repo.status.URL, chart)
		}
		// otherwise, chart URL is remote

		// Local-first: Check if chart archive is bundled locally in the checkout and matches digest.
		if localChart, err := git.LocalChart(namespace, name, repo.status.URL, chart); err == nil {
			// "cache hit"
			return localChart, nil
		} else if errors.Is(err, git.ErrNotCheckedOut) {
			// We must return when repository is not ready, because we cannot serve the chart from the local checkout.
			// This is the only option available for clusters with no internet access.
			return nil, notReady(err)
		} else if !errors.Is(err, validation.NotFound) {
			logrus.Debugf("local-first lookup for chart %s version %s in ClusterRepo %s not served locally: %v", chart.Name, chart.Version, name, err)
		}

		// NOTE: OCI chart URLs from a git-based ClusterRepo do not support authentication.
		// .spec.ClientSecret is overloaded, used to provide secrets for both git and OCI ClusterRepo types.
		// It is not safe to use this secret for OCI URLs in git-based repos.
		// We could make an exception with the system charts repository, and inject the default registry auth secret,
		// but that would too much anticipation, it is not necessary at the moment.
	} else {
		secret, err = catalogv2.GetSecret(c.secrets, repo.spec, repo.metadata.Namespace)
		if err != nil {
			return nil, err
		}
	}

	fetchRemote := func() (io.ReadCloser, error) {
		switch {
		case registry.IsOCI(chartURL):
			return oci.Chart(secret, chartURL, *repo.spec)
		default:
			return helmhttp.Chart(secret, repo.status.URL, repo.spec.CABundle, repo.spec.InsecureSkipTLSverify, repo.spec.DisableSameOriginCheck, chart)
		}
	}

	if !canUseCache {
		return fetchRemote()
	}

	if _, err, _ := c.sf.Do(digest, func() (any, error) {
		remoteReader, err := fetchRemote()
		if err != nil {
			return nil, err
		}
		defer remoteReader.Close()

		return nil, c.chartCache.Put(digest, remoteReader)
	}); err != nil {
		return nil, err
	}
	return c.chartCache.Get(digest)
}

// NotReadyRetryAfter is the delay clients are asked to wait, through Retry-After, before asking again
// for content of a repository that is still being checked out.
const NotReadyRetryAfter = 5 * time.Second

// notReady turns git.ErrNotCheckedOut into a 503, as the repository is expected to be restored by
// the ClusterRepo controller shortly, and leaves any other error as is. Use IsNotReady to tell this
// 503 apart from any other one.
func notReady(err error) error {
	if errors.Is(err, git.ErrNotCheckedOut) {
		return apierror.WrapAPIError(err, validation.ClusterUnavailable, err.Error())
	}
	return err
}

// IsNotReady returns true if err was caused by the repository not being checked out yet by this
// replica. Unlike other 503s, retrying after NotReadyRetryAfter is expected to succeed.
func IsNotReady(err error) bool {
	var apiErr *apierror.APIError
	return errors.As(err, &apiErr) && errors.Is(apiErr.Cause, git.ErrNotCheckedOut)
}

// isLocalChart reports whether a chart URL points to a file inside the local Git repository checkout.
// Relative paths, file:// URLs, and URLs matching the Git repository URL itself are resolved locally.
func isLocalChart(chartURL, gitURL string) bool {
	if !strings.Contains(chartURL, "://") || strings.HasPrefix(chartURL, "file://") {
		return true
	}
	if gitURL != "" && strings.HasPrefix(chartURL, gitURL) {
		return true
	}
	return false
}

// Info retrieves detailed information about a specific Helm chart from a Helm repository.
//
// The function uses the Chart method to get the content of the Helm chart.
// The Chart method is called with the skipFilter parameter hard-coded to true,
// meaning that no filtering is applied to the results.
//
// Once the chart content is retrieved, the function uses the InfoFromTarball method to extract detailed information.
//
// The function returns a types.ChartInfo pointer which represents the detailed information
// about the Helm chart and can be used by the Steve API.
func (c *Manager) Info(namespace, name, chartName, version string) (*types.ChartInfo, error) {
	chart, err := c.Chart(namespace, name, chartName, version, true)
	if err != nil {
		return nil, err
	}
	defer chart.Close()

	return helm.InfoFromTarball(chart)
}

// getRepo returns a cluster repository based on the name
//
// namespace should never be empty
//
// getRepo will get ClusterRepo struct defined for catalog.cattle.io, convert it to repoDef and return it
func (c *Manager) getRepo(namespace, name string) (repoDef, error) {
	if namespace == "" {
		cr, err := c.clusterRepos.Get(name)
		if err != nil {
			return repoDef{}, err
		}
		// A disabled repository behaves as if it didn't exist
		if cr.Spec.Enabled != nil && !*cr.Spec.Enabled {
			return repoDef{}, fmt.Errorf("clusterrepo %s is disabled: %w", name, validation.NotFound)
		}
		return repoDef{
			typedata: &cr.TypeMeta,
			metadata: &cr.ObjectMeta,
			spec:     &cr.Spec,
			status:   &cr.Status,
		}, nil
	}

	panic("namespace should never be empty")
}

// readBytes returns its "content" as a byte slice, concatenated with the content of any ConfigMaps linked to it via the "catalog.cattle.io/next" annotation.
func (c *Manager) readBytes(cm *corev1.ConfigMap) ([]byte, error) {
	var (
		bytes = cm.BinaryData["content"]
		err   error
	)

	for {
		next := cm.Annotations["catalog.cattle.io/next"]
		if next == "" {
			break
		}
		cm, err = c.configMaps.Get(cm.Namespace, next)
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, cm.BinaryData["content"]...)
	}

	return bytes, nil
}

// k8sVersion returns the Kubernetes version as a semver.Version struct.
func (c *Manager) k8sVersion() (*semver.Version, error) {
	info, err := c.discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	return semver.NewVersion(info.GitVersion)
}

// deepCopyIndex returns a new pointer of IndexFile that was deep copied
func deepCopyIndex(src *repo.IndexFile) *repo.IndexFile {
	deepcopy := repo.IndexFile{
		APIVersion: src.APIVersion,
		Generated:  src.Generated,
		Entries:    map[string]repo.ChartVersions{},
	}
	keys := deepcopy.PublicKeys
	copy(keys, src.PublicKeys)
	for k, entries := range src.Entries {
		for _, chart := range entries {
			cpMeta := *chart.Metadata
			cpChart := &repo.ChartVersion{
				Metadata: &cpMeta,
				Created:  chart.Created,
				Removed:  chart.Removed,
				Digest:   chart.Digest,
				URLs:     make([]string, len(chart.URLs)),
			}

			copy(cpChart.URLs, chart.URLs)
			deepcopy.Entries[k] = append(deepcopy.Entries[k], cpChart)
		}
	}
	return &deepcopy
}

// filterReleases filters out any chart versions that do not match the Rancher and Kubernetes versions, if specified in the chart's annotations.
// Returns the filtered or unfiltered IndexFile of a chart repository
func (c *Manager) filterReleases(index *repo.IndexFile, k8sVersion *semver.Version, skipFilter bool) *repo.IndexFile {
	if skipFilter || skipIndexFiltering() {
		return index
	}

	// get instance of rancher version and try to parse it
	rancherVersion, err := semver.NewVersion(settings.ServerVersion.Get())
	if err != nil {
		logrus.Errorf("failed to parse server version %s: %v", settings.ServerVersion.Get(), err)
		return index
	}
	rancherVersionWithoutPrerelease, err := rancherVersion.SetPrerelease("")
	if err != nil {
		logrus.Errorf("failed to remove prerelease from %s: %v", settings.ServerVersion.Get(), err)
		return index
	}

	for rel, versions := range index.Entries {
		newVersions := make([]*repo.ChartVersion, 0, len(versions))
		for _, version := range versions {
			if constraintStr, ok := version.Annotations["catalog.cattle.io/rancher-version"]; ok {
				if constraint, err := semver.NewConstraint(constraintStr); err == nil {
					satisfiesConstraint, errs := constraint.Validate(rancherVersion)
					// Check if the reason for failure is because it is ignroing prereleases
					constraintDoesNotMatchPrereleases := false
					for _, err := range errs {
						// Comes from error in https://github.com/Masterminds/semver/blob/60c7ae8a99210a90a9457d5de5f6dcbc4dab8e64/constraints.go#L93
						if strings.Contains(err.Error(), "the constraint is only looking for release versions") {
							constraintDoesNotMatchPrereleases = true
							break
						}
					}
					if constraintDoesNotMatchPrereleases {
						satisfiesConstraint = constraint.Check(&rancherVersionWithoutPrerelease)
					}
					if !satisfiesConstraint {
						continue
					}
				} else {
					logrus.Errorf("failed to parse constraint version %s: %v", constraintStr, err)
				}
			}
			if constraintStr, ok := version.Annotations["catalog.cattle.io/kube-version"]; ok {
				if constraint, err := semver.NewConstraint(constraintStr); err == nil {
					if !constraint.Check(k8sVersion) {
						continue
					}
				} else {
					logrus.Errorf("failed to parse constraint kube-version %s from annotation: %v", constraintStr, err)
				}
			}
			if version.KubeVersion != "" {
				if constraint, err := semver.NewConstraint(version.KubeVersion); err == nil {
					if !constraint.Check(k8sVersion) {
						continue
					}
				} else {
					logrus.Errorf("failed to parse constraint for kubeversion %s: %v", version.KubeVersion, err)
				}

			}
			newVersions = append(newVersions, version)
		}

		if len(newVersions) == 0 {
			delete(index.Entries, rel)
		} else {
			index.Entries[rel] = newVersions
		}
	}

	return index
}

// isHTTP - given a string, returns true if it is a valid HTTP or HTTPS URL; false otherwise.
func isHTTP(iconURL string) bool {
	u, err := url.Parse(iconURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// isRancherAndBundledCatalog - checks if the current chart repo
// is from the default rancher official helm catalog and if rancher is operating at bundled mode
// which means Rancher is at an airgapped environment
func isRancherAndBundledCatalog(repo repoDef) bool {
	gitDir := git.RepoDir(repo.metadata.Namespace, repo.metadata.Name, repo.status.URL)
	return (git.IsBundled(gitDir) && settings.SystemCatalog.Get() == "bundled")
}

// skipIndexFiltering returns true if the helm index filtering should be skipped.
func skipIndexFiltering() bool {
	serverVersion := settings.ServerVersion.Get()

	// If the server version is a release or a release candidate (rc), we don't skip filtering.
	if settings.IsRelease() || strings.Contains(serverVersion, "-rc") || strings.Contains(serverVersion, "-alpha") {
		return false
	}
	// If setting is true, we skip filtering.
	return settings.SkipHelmIndexFiltering.Get() == "true"
}
