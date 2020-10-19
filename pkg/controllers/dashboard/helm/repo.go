package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"strings"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	helmhttp "github.com/rancher/rancher/pkg/catalogv2/http"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/pkg/condition"
	corev1controllers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	interval = 5 * time.Minute
)

type repoHandler struct {
	secrets      corev1controllers.SecretCache
	clusterRepos catalogcontrollers.ClusterRepoController
	configMaps   corev1controllers.ConfigMapClient
}

func RegisterRepos(ctx context.Context,
	secrets corev1controllers.SecretCache,
	clusterRepos catalogcontrollers.ClusterRepoController,
	configMap corev1controllers.ConfigMapClient) {
	h := &repoHandler{
		secrets:      secrets,
		clusterRepos: clusterRepos,
		configMaps:   configMap,
	}

	catalogcontrollers.RegisterClusterRepoStatusHandler(ctx, clusterRepos,
		condition.Cond(catalog.RepoDownloaded), "helm-clusterrepo-download", h.ClusterRepoDownloadStatusHandler)

}

func RegisterReposForFollowers(ctx context.Context,
	secrets corev1controllers.SecretCache,
	clusterRepos catalogcontrollers.ClusterRepoController) {
	h := &repoHandler{
		secrets:      secrets,
		clusterRepos: clusterRepos,
	}

	catalogcontrollers.RegisterClusterRepoStatusHandler(ctx, clusterRepos,
		condition.Cond(catalog.FollowerRepoDownloaded), "helm-clusterrepo-ensure", h.ClusterRepoDownloadEnsureStatusHandler)

}

func (r *repoHandler) ClusterRepoDownloadEnsureStatusHandler(repo *catalog.ClusterRepo, status catalog.RepoStatus) (catalog.RepoStatus, error) {
	r.clusterRepos.EnqueueAfter(repo.Name, interval)
	return r.ensure(&repo.Spec, status, &repo.ObjectMeta)
}

func (r *repoHandler) ClusterRepoDownloadStatusHandler(repo *catalog.ClusterRepo, status catalog.RepoStatus) (catalog.RepoStatus, error) {
	if !shouldRefresh(&repo.Spec, &status) {
		r.clusterRepos.EnqueueAfter(repo.Name, interval)
		return status, nil
	}

	return r.download(&repo.Spec, status, &repo.ObjectMeta, metav1.OwnerReference{
		APIVersion: catalog.SchemeGroupVersion.Group + "/" + catalog.SchemeGroupVersion.Version,
		Kind:       "ClusterRepo",
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
		namespace = namespaces.System
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

func (r *repoHandler) ensure(repoSpec *catalog.RepoSpec, status catalog.RepoStatus, metadata *metav1.ObjectMeta) (catalog.RepoStatus, error) {
	if status.Commit == "" {
		return status, nil
	}

	status.ObservedGeneration = metadata.Generation
	secret, err := catalogv2.GetSecret(r.secrets, repoSpec, metadata.Namespace)
	if err != nil {
		return status, err
	}

	return status, git.Ensure(secret, metadata.Namespace, metadata.Name, status.URL, status.Commit)
}

func (r *repoHandler) download(repoSpec *catalog.RepoSpec, status catalog.RepoStatus, metadata *metav1.ObjectMeta, owner metav1.OwnerReference) (catalog.RepoStatus, error) {
	var (
		index  *repo.IndexFile
		commit string
		err    error
	)

	status.ObservedGeneration = metadata.Generation

	secret, err := catalogv2.GetSecret(r.secrets, repoSpec, metadata.Namespace)
	if err != nil {
		return status, err
	}

	downloadTime := metav1.Now()
	if repoSpec.GitRepo != "" && status.IndexConfigMapName == "" {
		commit, err = git.Head(secret, metadata.Namespace, metadata.Name, repoSpec.GitRepo, repoSpec.GitBranch)
		if err != nil {
			return status, err
		}
		status.URL = repoSpec.GitRepo
		status.Branch = repoSpec.GitBranch
		index, err = git.BuildOrGetIndex(metadata.Namespace, metadata.Name, repoSpec.GitRepo)
	} else if repoSpec.GitRepo != "" {
		commit, err = git.Update(secret, metadata.Namespace, metadata.Name, repoSpec.GitRepo, repoSpec.GitBranch)
		if err != nil {
			return status, err
		}
		status.URL = repoSpec.GitRepo
		status.Branch = repoSpec.GitBranch
		if status.Commit == commit {
			status.DownloadTime = downloadTime
			return status, nil
		}
		index, err = git.BuildOrGetIndex(metadata.Namespace, metadata.Name, repoSpec.GitRepo)
	} else if repoSpec.URL != "" {
		status.URL = strings.TrimSuffix(repoSpec.URL, "/") + "/"
		status.Branch = ""
		index, err = helmhttp.DownloadIndex(secret, repoSpec.URL, repoSpec.CABundle, repoSpec.InsecureSkipTLSverify)
	} else {
		return status, nil
	}
	if err != nil || index == nil {
		return status, err
	}

	index.SortEntries()

	name := status.IndexConfigMapName
	if name == "" {
		name = owner.Name
	}

	cm, err := r.createOrUpdateMap(metadata.Namespace, name, index, owner)
	if err != nil {
		return status, nil
	}

	status.IndexConfigMapName = cm.Name
	status.IndexConfigMapNamespace = cm.Namespace
	status.IndexConfigMapResourceVersion = cm.ResourceVersion
	status.DownloadTime = downloadTime
	status.Commit = commit
	return status, nil
}

func shouldRefresh(spec *catalog.RepoSpec, status *catalog.RepoStatus) bool {
	if status.Branch != spec.GitBranch {
		return true
	}
	if spec.URL != "" && spec.URL != status.URL {
		return true
	}
	if spec.GitRepo != "" && spec.GitRepo != status.URL {
		return true
	}
	if status.IndexConfigMapName == "" {
		return true
	}
	if spec.ForceUpdate != nil && spec.ForceUpdate.After(status.DownloadTime.Time) && spec.ForceUpdate.Time.Before(time.Now()) {
		return true
	}
	refreshTime := time.Now().Add(-interval)
	return refreshTime.After(status.DownloadTime.Time)
}
