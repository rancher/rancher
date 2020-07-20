package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/helm"
	"github.com/rancher/rancher/pkg/settings"
	"github.com/rancher/wrangler/pkg/condition"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var (
	interval               = 5 * time.Minute
	CatalogSystemNameSpace = "dashboard-catalog"
	repoUID                = "catalog.cattle.io/repo-uid"
)

type repoHandler struct {
	secrets      corev1controllers.SecretClient
	repos        catalogcontrollers.RepoController
	clusterRepos catalogcontrollers.ClusterRepoController
	configMaps   corev1controllers.ConfigMapClient
}

func RegisterRepos(ctx context.Context,
	secrets corev1controllers.SecretClient,
	repos catalogcontrollers.RepoController,
	clusterRepos catalogcontrollers.ClusterRepoController,
	configMap corev1controllers.ConfigMapClient) {
	h := &repoHandler{
		secrets:      secrets,
		repos:        repos,
		clusterRepos: clusterRepos,
		configMaps:   configMap,
	}

	catalogcontrollers.RegisterRepoStatusHandler(ctx, repos,
		condition.Cond(catalog.RepoDownloaded), "helm-repo-download", h.RepoDownloadStatusHandler)
	catalogcontrollers.RegisterClusterRepoStatusHandler(ctx, clusterRepos,
		condition.Cond(catalog.RepoDownloaded), "helm-clusterrepo-download", h.ClusterRepoDownloadStatusHandler)

}

func (r *repoHandler) ClusterRepoDownloadStatusHandler(repo *catalog.ClusterRepo, status catalog.RepoStatus) (catalog.RepoStatus, error) {
	if !shouldRefresh(&repo.Spec, &status) {
		r.clusterRepos.EnqueueAfter(repo.Name, interval)
		return status, nil
	}

	return r.download(&repo.Spec, status, "", metav1.OwnerReference{
		APIVersion: catalog.SchemeGroupVersion.Group + "/" + catalog.SchemeGroupVersion.Version,
		Kind:       "ClusterRepo",
		Name:       repo.Name,
		UID:        repo.UID,
	})
}

func (r *repoHandler) RepoDownloadStatusHandler(repo *catalog.Repo, status catalog.RepoStatus) (catalog.RepoStatus, error) {
	if !shouldRefresh(&repo.Spec, &status) {
		r.repos.EnqueueAfter(repo.Namespace, repo.Name, interval)
		return status, nil
	}

	return r.download(&repo.Spec, status, repo.Namespace, metav1.OwnerReference{
		APIVersion: catalog.SchemeGroupVersion.Group + "/" + catalog.SchemeGroupVersion.Version,
		Kind:       "Repo",
		Name:       repo.Name,
		UID:        repo.UID,
	})
}

func (r *repoHandler) createOrUpdateMap(namespace, name string, index *repo.IndexFile, owner metav1.OwnerReference) (*corev1.ConfigMap, error) {
	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if err := json.NewEncoder(gz).Encode(index); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = CatalogSystemNameSpace
	}

	cm, err := r.configMaps.Get(namespace, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if apierrors.IsNotFound(err) || len(cm.OwnerReferences) == 0 || cm.OwnerReferences[0].UID != owner.UID {
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName:    name + "-",
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{owner},
			},
			BinaryData: map[string][]byte{
				"content": buf.Bytes(),
			},
		}
		return r.configMaps.Create(cm)
	}

	cm.BinaryData = map[string][]byte{
		"content": buf.Bytes(),
	}
	return r.configMaps.Update(cm)
}

func (r *repoHandler) download(repoSpec *catalog.RepoSpec, status catalog.RepoStatus, secretNamespaceOverride string, owner metav1.OwnerReference) (catalog.RepoStatus, error) {
	client, err := helm.Client(r.secrets, repoSpec, secretNamespaceOverride)
	if err != nil {
		return status, err
	}
	defer client.CloseIdleConnections()

	parsedURL, err := url.Parse(repoSpec.URL)
	if err != nil {
		return status, err
	}

	parsedURL.RawPath = path.Join(parsedURL.RawPath, "index.yaml")
	parsedURL.Path = path.Join(parsedURL.Path, "index.yaml")

	downloadTime := metav1.Now()
	url := parsedURL.String()
	logrus.Infof("Downloading repo index from %s", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return status, err
	}
	req.Header.Set("X-Install-Uuid", settings.InstallUUID.Get())

	resp, err := client.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return status, err
	}

	// Marshall to file to ensure it matches the schema and this component doesn't just
	// become a "fetch any file" service.
	index := &repo.IndexFile{}
	if err := yaml.Unmarshal(bytes, index); err != nil {
		logrus.Errorf("failed to unmarshal %s: %v", url, err)
		return status, fmt.Errorf("failed to parse response from %s", url)
	}
	index.SortEntries()

	name := status.IndexConfigMapName
	if name == "" {
		name = owner.Name
	}

	cm, err := r.createOrUpdateMap(secretNamespaceOverride, name, index, owner)
	if err != nil {
		return status, nil
	}

	status.IndexConfigMapName = cm.Name
	status.IndexConfigMapNamespace = cm.Namespace
	status.IndexConfigMapResourceVersion = cm.ResourceVersion
	status.DownloadTime = downloadTime
	return status, nil
}

func shouldRefresh(spec *catalog.RepoSpec, status *catalog.RepoStatus) bool {
	if status.IndexConfigMapName == "" {
		return true
	}
	if spec.ForceUpdate != nil && spec.ForceUpdate.After(status.DownloadTime.Time) && spec.ForceUpdate.Time.Before(time.Now()) {
		return true
	}
	refreshTime := time.Now().Add(-interval)
	return refreshTime.After(status.DownloadTime.Time)
}
