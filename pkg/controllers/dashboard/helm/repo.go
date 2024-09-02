package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/api/equality"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2"
	"github.com/rancher/rancher/pkg/catalogv2/git"
	helmhttp "github.com/rancher/rancher/pkg/catalogv2/http"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/condition"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	name2 "github.com/rancher/wrangler/v3/pkg/name"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	maxSize         = 100_000
	defaultInterval = 6 * time.Hour
)

var defaultRetryPolicy = retryPolicy{
	MinWait:  5 * time.Minute,
	MaxWait:  20 * time.Minute,
	MaxRetry: 3,
}

type repoHandler struct {
	secrets        corev1controllers.SecretCache
	clusterRepos   catalogcontrollers.ClusterRepoController
	configMaps     corev1controllers.ConfigMapClient
	configMapCache corev1controllers.ConfigMapCache
	apply          apply.Apply
}

func RegisterRepos(ctx context.Context,
	apply apply.Apply,
	secrets corev1controllers.SecretCache,
	clusterRepos catalogcontrollers.ClusterRepoController,
	configMap corev1controllers.ConfigMapController,
	configMapCache corev1controllers.ConfigMapCache) {
	h := &repoHandler{
		secrets:        secrets,
		clusterRepos:   clusterRepos,
		configMaps:     configMap,
		configMapCache: configMapCache,
		apply:          apply.WithCacheTypes(configMap).WithStrictCaching().WithSetOwnerReference(false, false),
	}

	clusterRepos.OnChange(ctx, "helm-clusterrepo-download-on-change", h.ClusterRepoDownloadStatusHandler2)

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
	if registry.IsOCI(repo.Spec.URL) {
		return status, nil
	}
	r.clusterRepos.EnqueueAfter(repo.Name, defaultInterval)
	return r.ensure(&repo.Spec, status, &repo.ObjectMeta)
}

func (r *repoHandler) ClusterRepoDownloadStatusHandler2(key string, repo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	// Ignore OCI Based Helm Repositories
	if registry.IsOCI(repo.Spec.URL) {
		return repo, nil
	}

	interval := defaultInterval

	newStatus := repo.Status.DeepCopy()
	retryPolicy, err := getRetryPolicy(repo)
	if err != nil {
		err = fmt.Errorf("failed to get retry policy: %w", err)
		return r.setErrorCondition(repo, err, newStatus, interval)
	}
	if r.shouldSkip(repo, retryPolicy, repo.Name, interval) {
		return repo, nil
	}
	newStatus.ShouldNotSkip = false

	err = ensureIndexConfigMap(repo, &repo.Status, r.configMaps)
	if err != nil {
		err = fmt.Errorf("failed to ensure index config map: %w", err)
		return r.setErrorCondition(repo, err, newStatus, interval)
	}

	if !shouldRefresh(&repo.Spec, &repo.Status, interval) { //reset retries too
		r.clusterRepos.EnqueueAfter(repo.Name, interval)
		return repo, nil
	}

	return r.download(repo, *newStatus, metav1.OwnerReference{
		APIVersion: catalog.SchemeGroupVersion.Group + "/" + catalog.SchemeGroupVersion.Version,
		Kind:       "ClusterRepo",
		Name:       repo.Name,
		UID:        repo.UID,
	}, interval, retryPolicy)
}

func toOwnerObject(namespace string, owner metav1.OwnerReference) runtime.Object {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       owner.Kind,
			APIVersion: owner.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      owner.Name,
			Namespace: namespace,
			UID:       owner.UID,
		},
	}
}

func createOrUpdateMap(namespace string, index *repo.IndexFile, owner metav1.OwnerReference, apply apply.Apply) (*corev1.ConfigMap, error) {
	// do this before we normalize the namespace
	ownerObject := toOwnerObject(namespace, owner)

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if err := json.NewEncoder(gz).Encode(index); err != nil {
		logrus.Errorf("error while encoding index: %v", err)
		return nil, err
	}
	if err := gz.Close(); err != nil {
		logrus.Errorf("error while closing reader: %v", err)
		return nil, err
	}

	namespace = GetConfigMapNamespace(namespace)

	var (
		objs  []runtime.Object
		bytes = buf.Bytes()
		left  []byte
		i     = 0
		size  = len(bytes)
	)

	for {
		if len(bytes) > maxSize {
			left = bytes[maxSize:]
			bytes = bytes[:maxSize]
		}

		next := ""
		if len(left) > 0 {
			next = GenerateConfigMapName(owner.Name, i+1, owner.UID)
		}

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            GenerateConfigMapName(owner.Name, i, owner.UID),
				Namespace:       namespace,
				OwnerReferences: []metav1.OwnerReference{owner},
				Annotations: map[string]string{
					"catalog.cattle.io/next": next,
					// Size ensure the resource version should update even if this is the head of a multipart chunk
					"catalog.cattle.io/size": fmt.Sprint(size),
				},
			},
			BinaryData: map[string][]byte{
				"content": bytes,
			},
		}

		objs = append(objs, cm)
		if len(left) == 0 {
			break
		}

		i++
		bytes = left
		left = nil
	}
	err := apply.WithOwner(ownerObject).ApplyObjects(objs...)
	if err != nil {
		logrus.Errorf("error while applying configmap %s: %v", GenerateConfigMapName(owner.Name, i, owner.UID), err)
	}
	return objs[0].(*corev1.ConfigMap), err
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

	return status, git.Ensure(secret, metadata.Namespace, metadata.Name, status.URL, status.Commit, repoSpec.InsecureSkipTLSverify, repoSpec.CABundle)
}

func (r *repoHandler) download(repository *catalog.ClusterRepo, newStatus catalog.RepoStatus, owner metav1.OwnerReference, interval time.Duration, retryPolicy retryPolicy) (*catalog.ClusterRepo, error) {
	var (
		index  *repo.IndexFile
		commit string
		err    error
	)

	metadata := repository.ObjectMeta
	repoSpec := repository.Spec
	newStatus.ObservedGeneration = metadata.Generation

	secret, err := catalogv2.GetSecret(r.secrets, &repoSpec, metadata.Namespace)
	if err != nil {
		return r.setErrorCondition(repository, err, &newStatus, interval)
	}

	downloadTime := metav1.Now()
	backoff := calculateBackoff(repository, retryPolicy)
	retriable := false
	// git repo and no configmap
	if repoSpec.GitRepo != "" && newStatus.IndexConfigMapName == "" {
		//retry here
		commit, err = git.Head(secret, metadata.Namespace, metadata.Name, repoSpec.GitRepo, repoSpec.GitBranch, repoSpec.InsecureSkipTLSverify, repoSpec.CABundle)
		if err != nil {
			retriable = true
		} else {
			newStatus.URL = repoSpec.GitRepo
			newStatus.Branch = repoSpec.GitBranch
			//no retry here, no external calls
			index, err = git.BuildOrGetIndex(metadata.Namespace, metadata.Name, repoSpec.GitRepo)
		}
		//git repo
	} else if repoSpec.GitRepo != "" {
		//retry here
		commit, err = git.Update(secret, metadata.Namespace, metadata.Name, repoSpec.GitRepo, repoSpec.GitBranch, repoSpec.InsecureSkipTLSverify, repoSpec.CABundle)
		if err != nil {
			retriable = true
			//return status, err
		} else {
			newStatus.URL = repoSpec.GitRepo
			newStatus.Branch = repoSpec.GitBranch
			if newStatus.Commit == commit {
				newStatus.DownloadTime = downloadTime
				return repository, nil
			}
			//no retry here, no external calls
			index, err = git.BuildOrGetIndex(metadata.Namespace, metadata.Name, repoSpec.GitRepo)
		}
		// http repo
	} else if repoSpec.URL != "" {
		//retry here
		index, err = helmhttp.DownloadIndex(secret, repoSpec.URL, repoSpec.CABundle, repoSpec.InsecureSkipTLSverify, repoSpec.DisableSameOriginCheck)
		retriable = true
		newStatus.URL = repoSpec.URL
		newStatus.Branch = ""
		// something weird
	} else {
		return repository, nil
	}
	if retriable && err != nil {
		newStatus.NumberOfRetries++
		if newStatus.NumberOfRetries > retryPolicy.MaxRetry {
			newStatus.NumberOfRetries = 0
			newStatus.NextRetryAt = metav1.Time{}
			return r.setConditionWithInterval(repository, err, &newStatus, nil, interval)
		}
		newStatus.NextRetryAt = metav1.Time{Time: timeNow().UTC().Add(backoff)}
		return r.setConditionWithInterval(repository, err, &newStatus, &backoff, interval)
	}
	if err != nil {
		return repository, err
	}
	if index == nil {
		return repository, nil
	}

	index.SortEntries()
	cm, err := createOrUpdateMap(metadata.Namespace, index, owner, r.apply)
	if err != nil {
		return repository, err
	}

	newStatus.IndexConfigMapName = cm.Name
	newStatus.IndexConfigMapNamespace = cm.Namespace
	newStatus.IndexConfigMapResourceVersion = cm.ResourceVersion
	newStatus.DownloadTime = downloadTime
	newStatus.Commit = commit
	repository.Status = newStatus
	return r.clusterRepos.UpdateStatus(repository)
}

func ensureIndexConfigMap(repo *catalog.ClusterRepo, status *catalog.RepoStatus, configMap corev1controllers.ConfigMapClient) error {
	// Charts from the clusterRepo will be unavailable if the IndexConfigMap recorded in the status does not exist.
	// By resetting the value of IndexConfigMapName, IndexConfigMapNamespace, IndexConfigMapResourceVersion to "",
	// the method shouldRefresh will return true and trigger the rebuild of the IndexConfigMap and accordingly update the status.
	if repo.Spec.GitRepo != "" && status.IndexConfigMapName != "" {
		_, err := configMap.Get(status.IndexConfigMapNamespace, status.IndexConfigMapName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				status.IndexConfigMapName = ""
				status.IndexConfigMapNamespace = ""
				status.IndexConfigMapResourceVersion = ""
				return nil
			}
			logrus.Errorf("Error while fetching index config map %s : %v", status.IndexConfigMapName, err)
			reason := apierrors.ReasonForError(err)
			if reason == metav1.StatusReasonUnknown {
				return err
			}
			return fmt.Errorf("failed to fetch index config map for cluster repo: %s", reason)
		}
	}
	return nil
}

func shouldRefresh(spec *catalog.RepoSpec, status *catalog.RepoStatus, interval time.Duration) bool {
	// check if branch changed
	if spec.GitRepo != "" && status.Branch != spec.GitBranch {
		return true
	}
	// check if url changed
	if spec.URL != "" && spec.URL != status.URL {
		return true
	}
	// check if git repo changed
	if spec.GitRepo != "" && spec.GitRepo != status.URL {
		return true
	}
	// check if there's no index config map
	if status.IndexConfigMapName == "" {
		return true
	}
	// check if it's a force update
	if spec.ForceUpdate != nil && spec.ForceUpdate.After(status.DownloadTime.Time) && spec.ForceUpdate.Time.Before(time.Now()) {
		return true
	}
	refreshTime := time.Now().Add(-interval)
	// check if interval has passed
	return refreshTime.After(status.DownloadTime.Time)
}

func GenerateConfigMapName(ownerName string, index int, UID types.UID) string {
	return name2.SafeConcatName(ownerName, fmt.Sprint(index), string(UID))
}

func GetConfigMapNamespace(namespace string) string {
	if namespace == "" {
		return namespaces.System
	}

	return namespace
}

// setErrorCondition is only called when error happens in the handler, and
// we need to depend on wrangler to requeue the handler
func (r *repoHandler) setErrorCondition(clusterRepo *catalog.ClusterRepo, err error, newStatus *catalog.RepoStatus, interval time.Duration) (*catalog.ClusterRepo, error) {
	var statusErr error
	newStatus.NumberOfRetries = 0
	newStatus.NextRetryAt = metav1.Time{}
	if err != nil {
		newStatus.ShouldNotSkip = true
	}
	downloaded := condition.Cond(catalog.RepoDownloaded)
	if apierrors.IsConflict(err) {
		downloaded.SetError(newStatus, "", nil)
	} else {
		downloaded.SetError(newStatus, "", err)
	}
	newStatus.ObservedGeneration = clusterRepo.Generation

	if !equality.Semantic.DeepEqual(newStatus, &clusterRepo.Status) {
		downloaded.LastUpdated(newStatus, timeNow().UTC().Format(time.RFC3339))

		clusterRepo.Status = *newStatus
		//status handler will run again without waiting and enqueue after won't work
		clusterRepo, statusErr = r.clusterRepos.UpdateStatus(clusterRepo)
		if statusErr != nil {
			err = statusErr
		}
		if err == nil {
			r.clusterRepos.EnqueueAfter(clusterRepo.Name, interval)
		}
		return clusterRepo, err
	}

	if err == nil {
		r.clusterRepos.EnqueueAfter(clusterRepo.Name, interval)
	}
	return clusterRepo, err
}

// shouldSkip checks certain conditions to see if the handler should be skipped.
// For information regarding the conditions, check the comments in the implementation.
func (r *repoHandler) shouldSkip(clusterRepo *catalog.ClusterRepo, policy retryPolicy, key string, ociInterval time.Duration) bool {
	// this is to prevent the handler from making calls when the crd is outdated.
	updatedRepo, err := r.clusterRepos.Get(key, metav1.GetOptions{})
	if err == nil && updatedRepo.ResourceVersion != clusterRepo.ResourceVersion {
		return true
	}

	if clusterRepo.Status.ObservedGeneration < clusterRepo.Generation {
		clusterRepo.Status.NumberOfRetries = 0
		clusterRepo.Status.NextRetryAt = metav1.Time{}
		return false
	}

	// The handler is triggered immediately after any changes, including when updating the number of retries done.
	// This check is to prevent the handler from executing before the backoff time has passed
	if !clusterRepo.Status.NextRetryAt.IsZero() && clusterRepo.Status.NextRetryAt.Time.After(timeNow().UTC()) {
		return true
	}

	if clusterRepo.Status.ShouldNotSkip { //checks if we should skip running the handler or not
		return false
	}

	downloaded := condition.Cond(catalog.RepoDownloaded)
	downloadedUpdateTime, _ := time.Parse(time.RFC3339, downloaded.GetLastUpdated(clusterRepo))

	if (clusterRepo.Status.NumberOfRetries > policy.MaxRetry || clusterRepo.Status.NumberOfRetries == 0) && // checks if it's not retrying
		clusterRepo.Generation == clusterRepo.Status.ObservedGeneration && // checks if the generation has not changed
		downloadedUpdateTime.Add(ociInterval).After(timeNow().UTC()) { // checks if the interval has not passed

		r.clusterRepos.EnqueueAfter(clusterRepo.Name, ociInterval)
		return true
	}
	return false
}

// setConditionWithInterval is called to reenqueue the object
// after the interval of 6 hours.
func (r *repoHandler) setConditionWithInterval(clusterRepo *catalog.ClusterRepo, err error, newStatus *catalog.RepoStatus, backoff *time.Duration, interval time.Duration) (*catalog.ClusterRepo, error) {
	var newErr error
	if backoff != nil {
		newErr = fmt.Errorf("%s. %s", err.Error(), fmt.Sprintf("Will retry after %s", backoff.Round(time.Second)))
	} else {
		newErr = fmt.Errorf("%s. %s", err.Error(), fmt.Sprintf("Will retry after %s", interval.Round(time.Second)))
	}

	downloaded := condition.Cond(catalog.RepoDownloaded)
	if apierrors.IsConflict(err) {
		downloaded.SetError(newStatus, "", nil)
	} else {
		downloaded.SetError(newStatus, "", newErr)
	}
	newStatus.ObservedGeneration = clusterRepo.Generation
	if !equality.Semantic.DeepEqual(newStatus, &clusterRepo.Status) {
		//Since status has changed, update the lastUpdatedTime
		downloaded.LastUpdated(newStatus, timeNow().UTC().Format(time.RFC3339))
		clusterRepo.Status = *newStatus
		_, statusErr := r.clusterRepos.UpdateStatus(clusterRepo)
		if statusErr != nil {
			return clusterRepo, statusErr
		}
	}

	if backoff != nil {
		r.clusterRepos.EnqueueAfter(clusterRepo.Name, *backoff)
	} else {
		r.clusterRepos.EnqueueAfter(clusterRepo.Name, interval)
	}
	return clusterRepo, nil
}
