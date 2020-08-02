package content

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/url"

	"github.com/rancher/rancher/pkg/api/steve/catalog/types"
	v1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	"github.com/rancher/rancher/pkg/catalogv2/helm"
	helmhttp "github.com/rancher/rancher/pkg/catalogv2/http"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type Manager struct {
	configMaps   corecontrollers.ConfigMapCache
	repos        catalogcontrollers.RepoCache
	secrets      corecontrollers.SecretCache
	clusterRepos catalogcontrollers.ClusterRepoCache
}

func NewManager(
	configMaps corecontrollers.ConfigMapCache,
	repos catalogcontrollers.RepoCache,
	secrets corecontrollers.SecretCache,
	clusterRepos catalogcontrollers.ClusterRepoCache) *Manager {
	return &Manager{
		configMaps:   configMaps,
		repos:        repos,
		secrets:      secrets,
		clusterRepos: clusterRepos,
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

	cr, err := c.repos.Get(namespace, name)
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

func (c *Manager) Index(namespace, name string) (*repo.IndexFile, error) {
	r, err := c.getRepo(namespace, name)
	if err != nil {
		return nil, err
	}

	cm, err := c.configMaps.Get(r.status.IndexConfigMapNamespace, r.status.IndexConfigMapName)
	if err != nil {
		return nil, err
	}

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
	return index, yaml.Unmarshal(data, index)
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
