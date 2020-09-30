package content

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	"github.com/rancher/rancher/pkg/catalogv2/helm"
	helmhttp "github.com/rancher/rancher/pkg/catalogv2/http"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/settings"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

type Manager struct {
	configMaps   corecontrollers.ConfigMapCache
	secrets      corecontrollers.SecretCache
	clusterRepos catalogcontrollers.ClusterRepoCache
	discovery    discovery.DiscoveryInterface
	IndexCache   map[string]indexCache
	lock         sync.RWMutex
}

type indexCache struct {
	index    *repo.IndexFile
	revision string
}

func NewManager(
	discovery discovery.DiscoveryInterface,
	configMaps corecontrollers.ConfigMapCache,
	secrets corecontrollers.SecretCache,
	clusterRepos catalogcontrollers.ClusterRepoCache) *Manager {
	return &Manager{
		discovery:    discovery,
		configMaps:   configMaps,
		secrets:      secrets,
		clusterRepos: clusterRepos,
		IndexCache:   map[string]indexCache{},
	}
}

type repoDef struct {
	typedata *metav1.TypeMeta
	metadata *metav1.ObjectMeta
	spec     *v1.RepoSpec
	status   *v1.RepoStatus
}

func (c *Manager) getRepo(namespace, name string) (repoDef, error) {
	if namespace == "" {
		cr, err := c.clusterRepos.Get(name)
		if err != nil {
			return repoDef{}, err
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

func (c *Manager) Index(namespace, name string) (*repo.IndexFile, error) {
	r, err := c.getRepo(namespace, name)
	if err != nil {
		return nil, err
	}

	cm, err := c.configMaps.Get(r.status.IndexConfigMapNamespace, r.status.IndexConfigMapName)
	if err != nil {
		return nil, err
	}

	k8sVersion, err := c.k8sVersion()
	if err != nil {
		return nil, err
	}

	c.lock.RLock()
	if cache, ok := c.IndexCache[fmt.Sprintf("%s/%s", r.status.IndexConfigMapNamespace, r.status.IndexConfigMapName)]; ok {
		if cm.ResourceVersion == cache.revision {
			c.lock.RUnlock()
			return c.filterReleases(deepCopyIndex(cache.index), k8sVersion), nil
		}
	}
	c.lock.RUnlock()

	if len(cm.OwnerReferences) == 0 || cm.OwnerReferences[0].UID != r.metadata.UID {
		return nil, validation.Unauthorized
	}

	gz, err := gzip.NewReader(bytes.NewBuffer(cm.BinaryData["content"]))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	data, err := ioutil.ReadAll(gz)
	if err != nil {
		return nil, err
	}

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

	return c.filterReleases(deepCopyIndex(index), k8sVersion), nil
}

func (c *Manager) k8sVersion() (*semver.Version, error) {
	info, err := c.discovery.ServerVersion()
	if err != nil {
		return nil, err
	}
	return semver.NewVersion(info.GitVersion)
}

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

func (c *Manager) filterReleases(index *repo.IndexFile, k8sVersion *semver.Version) *repo.IndexFile {
	if !settings.IsRelease() {
		return index
	}

	rancherVersion, err := semver.NewVersion(settings.ServerVersion.Get())
	if err != nil {
		logrus.Errorf("failed to parse server version %s: %v", settings.ServerVersion.Get(), err)
		return index
	}

	for rel, versions := range index.Entries {
		newVersions := make([]*repo.ChartVersion, 0, len(versions))
		for _, version := range versions {
			if constraintStr, ok := version.Annotations["catalog.cattle.io/rancher-version"]; ok {
				if constraint, err := semver.NewConstraint(constraintStr); err == nil {
					if !constraint.Check(rancherVersion) {
						continue
					}
				} else {
					logrus.Errorf("failed to parse constraint version %s: %v", constraintStr, err)
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

func (c *Manager) Icon(namespace, name, chartName, version string) (io.ReadCloser, string, error) {
	index, err := c.Index(namespace, name)
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

	if !isHTTP(chart.Icon) && repo.status.Commit != "" {
		return git.Icon(namespace, name, repo.status.URL, chart)
	}

	secret, err := catalogv2.GetSecret(c.secrets, repo.spec, repo.metadata.Namespace)
	if err != nil {
		return nil, "", err
	}

	return helmhttp.Icon(secret, repo.status.URL, repo.spec.CABundle, repo.spec.InsecureSkipTLSverify, chart)
}

func isHTTP(iconURL string) bool {
	u, err := url.Parse(iconURL)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

func (c *Manager) Chart(namespace, name, chartName, version string) (io.ReadCloser, error) {
	index, err := c.Index(namespace, name)
	if err != nil {
		return nil, err
	}

	chart, err := index.Get(chartName, version)
	if err != nil {
		return nil, err
	}

	repo, err := c.getRepo(namespace, name)
	if err != nil {
		return nil, err
	}

	if repo.status.Commit != "" {
		return git.Chart(namespace, name, repo.status.URL, chart)
	}

	secret, err := catalogv2.GetSecret(c.secrets, repo.spec, repo.metadata.Namespace)
	if err != nil {
		return nil, err
	}

	return helmhttp.Chart(secret, repo.status.URL, repo.spec.CABundle, repo.spec.InsecureSkipTLSverify, chart)
}

func (c *Manager) Info(namespace, name, chartName, version string) (*types.ChartInfo, error) {
	chart, err := c.Chart(namespace, name, chartName, version)
	if err != nil {
		return nil, err
	}
	defer chart.Close()

	return helm.InfoFromTarball(chart)
}
