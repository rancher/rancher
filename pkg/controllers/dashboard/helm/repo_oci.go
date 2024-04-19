package helm

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/maphash"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"

	catalog "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
	"github.com/rancher/rancher/pkg/catalogv2"
	"github.com/rancher/rancher/pkg/catalogv2/oci"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/rancher/wrangler/v2/pkg/condition"
	corev1controllers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

var timeNow = time.Now

type OCIRepohandler struct {
	clusterRepoController catalogcontrollers.ClusterRepoController
	configMapController   corev1controllers.ConfigMapController
	secretCacheController corev1controllers.SecretCache
	apply                 apply.Apply
}

type retryPolicy struct {
	// MinWait is the minimum duration to wait before retrying.
	MinWait time.Duration

	// MaxWait is the maximum duration to wait before retrying.
	MaxWait time.Duration

	// MaxRetry is the maximum number of retries.
	MaxRetry int
}

func RegisterOCIRepo(ctx context.Context,
	apply apply.Apply,
	clusterRepoController catalogcontrollers.ClusterRepoController,
	configMapController corev1controllers.ConfigMapController,
	secretsController corev1controllers.SecretCache) {

	ociRepoHandler := &OCIRepohandler{
		clusterRepoController: clusterRepoController,
		configMapController:   configMapController,
		secretCacheController: secretsController,
		apply:                 apply.WithCacheTypes(configMapController).WithStrictCaching().WithSetOwnerReference(false, false),
	}

	clusterRepoController.OnChange(ctx, "oci-clusterrepo-helm", ociRepoHandler.onClusterRepoChange)
}

// This handler is retriggerd in the following cases
// * When the spec of the ClusterRepo is changed.
// * When there is no error from the handler, at a regular interval of 6 hours.
// * When there is an error from the handler, at the wrangler's default error interval.
// * When the response from OCI registry is anything 4xx HTTP status code, at an interval of 6 hours.
func (o *OCIRepohandler) onClusterRepoChange(key string, clusterRepo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	if clusterRepo == nil {
		return nil, nil
	}
	// Ignore non OCI ClusterRepos
	if !registry.IsOCI(clusterRepo.Spec.URL) {
		return clusterRepo, nil
	}

	// this is to prevent the handler from making calls when the crd is outdated.
	updatedRepo, err := o.clusterRepoController.Get(key, metav1.GetOptions{})
	if err == nil && updatedRepo.ResourceVersion != clusterRepo.ResourceVersion {
		return clusterRepo, nil
	}

	if shouldResetRetries(clusterRepo) {
		clusterRepo.Status.NumberOfRetries = 0
		clusterRepo.Status.NextRetryAt = metav1.Time{}
	}

	if !clusterRepo.Status.NextRetryAt.IsZero() && clusterRepo.Status.NextRetryAt.Time.After(timeNow()) {
		return clusterRepo, nil
	}

	retryPolicy := getRetryPolicy(clusterRepo)

	if clusterRepo.Status.NumberOfRetries > retryPolicy.MaxRetry {
		return clusterRepo, nil
	}

	originalStatus := clusterRepo.Status.DeepCopy()

	logrus.Debugf("OCIRepoHandler triggered for clusterrepo %s", clusterRepo.Name)
	var index *repo.IndexFile

	err = ensureIndexConfigMap(clusterRepo, originalStatus, o.configMapController)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}

	originalStatus.ObservedGeneration = clusterRepo.Generation
	secret, err := catalogv2.GetSecret(o.secretCacheController, &clusterRepo.Spec, clusterRepo.Namespace)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}

	owner := metav1.OwnerReference{
		APIVersion: catalog.SchemeGroupVersion.Group + "/" + catalog.SchemeGroupVersion.Version,
		Kind:       "ClusterRepo",
		Name:       clusterRepo.Name,
		UID:        clusterRepo.UID,
	}

	downloadTime := metav1.Now()
	index, err = getIndexfile(clusterRepo.Status, clusterRepo.Spec, o.configMapController, owner, clusterRepo.Namespace)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}
	originalIndexBytes, err := json.Marshal(index)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}
	index, err = oci.GenerateIndex(clusterRepo.Spec.URL, secret, clusterRepo.Spec, *originalStatus, index)
	// If there is 401 or 403 error code, then we don't reconcile further and wait for 6 hours interval
	var errResp *errcode.ErrorResponse
	if errors.As(err, &errResp) {
		if errResp.StatusCode == http.StatusUnauthorized ||
			errResp.StatusCode == http.StatusForbidden ||
			errResp.StatusCode == http.StatusNotFound {
			return o.set4xxCondition(clusterRepo, errResp, originalStatus)
		}

		// If there is 429 error code and max retry is reached, then we don't reconcile further and wait for 6 hours interval,
		// but we also create the configmap for future usecases.
		if errResp.StatusCode == http.StatusTooManyRequests {
			if index != nil && len(index.Entries) > 0 {
				originalStatus.URL = clusterRepo.Spec.URL

				index.SortEntries()
				_, err := createOrUpdateMap(clusterRepo.Namespace, index, owner, o.apply)
				if err != nil {
					logrus.Debugf("failed to create/udpate the configmap incase of 4xx statuscode for %s", clusterRepo.Name)
				}
			}

			clusterRepo.Status.NumberOfRetries++
			if clusterRepo.Status.NumberOfRetries > retryPolicy.MaxRetry {
				return o.set4xxCondition(clusterRepo, errResp, originalStatus)
			}

			backoff := calculateBackoff(clusterRepo, retryPolicy)

			clusterRepo.Status.NextRetryAt = metav1.Time{Time: timeNow().Add(backoff)}
			//updating the status triggers the handler again
			status, err := o.clusterRepoController.UpdateStatus(clusterRepo)
			if err != nil {
				return clusterRepo, err
			}
			o.clusterRepoController.EnqueueAfter(clusterRepo.Name, backoff)
			return status, nil
		}
	}
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}
	if index == nil || len(index.Entries) <= 0 {
		err = errors.New("there are no helm charts in the repository specified")
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}

	newIndexBytes, err := json.Marshal(index)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, originalStatus)
	}
	// Only update, if the index got updated
	if !bytes.Equal(originalIndexBytes, newIndexBytes) {
		index.SortEntries()
		cm, err := createOrUpdateMap(clusterRepo.Namespace, index, owner, o.apply)
		if err != nil {
			return o.setErrorCondition(clusterRepo, err, originalStatus)
		}

		originalStatus.URL = clusterRepo.Spec.URL
		originalStatus.IndexConfigMapName = cm.Name
		originalStatus.IndexConfigMapNamespace = cm.Namespace
		originalStatus.IndexConfigMapResourceVersion = cm.ResourceVersion
		originalStatus.DownloadTime = downloadTime
		originalStatus.NumberOfRetries = 0
	}

	return o.setErrorCondition(clusterRepo, err, originalStatus)
}

// setErrorCondition is only called when error happens in the handler, and
// we need to depend on wrangler to reenqueue the handler
func (o *OCIRepohandler) setErrorCondition(clusterRepo *catalog.ClusterRepo, err error, originalStatus *catalog.RepoStatus) (*catalog.ClusterRepo, error) {
	var statusErr error
	originalStatus.NumberOfRetries = 0
	originalStatus.NextRetryAt = metav1.Time{}

	ociDownloaded := condition.Cond(catalog.OCIDownloaded)
	if apierrors.IsConflict(err) {
		ociDownloaded.SetError(originalStatus, "", nil)
	} else {
		ociDownloaded.SetError(originalStatus, "", err)
	}

	if !equality.Semantic.DeepEqual(originalStatus, &clusterRepo.Status) {
		ociDownloaded.LastUpdated(originalStatus, time.Now().UTC().Format(time.RFC3339))

		clusterRepo.Status = *originalStatus
		clusterRepo, statusErr = o.clusterRepoController.UpdateStatus(clusterRepo)
		if statusErr != nil {
			err = statusErr
		}

		return clusterRepo, err
	}

	o.clusterRepoController.EnqueueAfter(clusterRepo.Name, interval)
	return clusterRepo, nil
}

// set4xxCondition is only called when we receive a 4xx error
// we need to wait for 6 hours to reenqueue.
func (o *OCIRepohandler) set4xxCondition(clusterRepo *catalog.ClusterRepo, err *errcode.ErrorResponse, originalStatus *catalog.RepoStatus) (*catalog.ClusterRepo, error) {
	err.Errors = append(err.Errors, errcode.Error{Message: fmt.Sprintf(" will retry will after %s", interval)})
	ociDownloaded := condition.Cond(catalog.OCIDownloaded)
	if apierrors.IsConflict(err) {
		ociDownloaded.SetError(originalStatus, "", nil)
	} else {
		ociDownloaded.SetError(originalStatus, "", err)
	}

	if !equality.Semantic.DeepEqual(originalStatus, &clusterRepo.Status) {
		// Since status has changed, update the lastUpdatedTime
		ociDownloaded.LastUpdated(originalStatus, time.Now().UTC().Format(time.RFC3339))

		originalStatus.NumberOfRetries = clusterRepo.Status.NumberOfRetries
		clusterRepo.Status = *originalStatus

		return o.clusterRepoController.UpdateStatus(clusterRepo)
	}

	o.clusterRepoController.EnqueueAfter(clusterRepo.Name, interval)
	return clusterRepo, nil
}

// getIndexfile fetches the indexfile if it already exits for the clusterRepo
// if not, it creates a new indexfile and returns it.
func getIndexfile(clusterRepoStatus catalog.RepoStatus,
	clusterRepoSpec catalog.RepoSpec,
	configMapClient corev1controllers.ConfigMapClient,
	owner metav1.OwnerReference,
	namespace string) (*repo.IndexFile, error) {

	indexFile := repo.NewIndexFile()
	var configMap *corev1.ConfigMap
	var err error
	if clusterRepoSpec.URL != clusterRepoStatus.URL {
		return indexFile, nil
	}

	// If the status has the configmap defined, fetch it.
	if clusterRepoStatus.IndexConfigMapName != "" {
		configMap, err = configMapClient.Get(clusterRepoStatus.IndexConfigMapNamespace, clusterRepoStatus.IndexConfigMapName, metav1.GetOptions{})
		if err != nil {
			return indexFile, fmt.Errorf("failed to fetch the index configmap for clusterRepo %s", owner.Name)
		}
	} else {
		// otherwise if the configmap is already created, fetch it using the name of the configmap and the namespace.
		configMapName := GenerateConfigMapName(owner.Name, 0, owner.UID)
		configMapNamespace := GetConfigMapNamespace(namespace)

		configMap, err = configMapClient.Get(configMapNamespace, configMapName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return indexFile, nil
			}
			return indexFile, fmt.Errorf("failed to fetch the index configmap for clusterRepo %s", owner.Name)
		}
	}

	data, err := readBytes(configMapClient, configMap)
	if err != nil {
		return indexFile, fmt.Errorf("failed to read bytes of existing configmap for URL %s", clusterRepoSpec.URL)
	}
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return indexFile, err
	}
	defer gz.Close()
	data, err = io.ReadAll(gz)
	if err != nil {
		return indexFile, err
	}
	if err := json.Unmarshal(data, indexFile); err != nil {
		return indexFile, err
	}

	return indexFile, nil
}

// readBytes reads data from the chain of helm repo index configmaps.
func readBytes(configMapCache corev1controllers.ConfigMapClient, cm *corev1.ConfigMap) ([]byte, error) {
	var (
		bytes = cm.BinaryData["content"]
		err   error
	)

	for {
		next := cm.Annotations["catalog.cattle.io/next"]
		if next == "" {
			break
		}
		cm, err = configMapCache.Get(cm.Namespace, next, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		bytes = append(bytes, cm.BinaryData["content"]...)
	}

	return bytes, nil
}

// calculateBackoff gets the amount of time to wait for the next call.
// Reference: https://github.com/oras-project/oras-go/blob/main/registry/remote/retry/policy.go#L95
func calculateBackoff(clusterRepo *catalog.ClusterRepo, policy retryPolicy) time.Duration {
	var h maphash.Hash
	h.SetSeed(maphash.MakeSeed())
	rand := rand.New(rand.NewSource(int64(h.Sum64())))
	temp := float64(policy.MinWait) * math.Pow(2, float64(clusterRepo.Status.NumberOfRetries))
	backoff := time.Duration(temp*(1-0.2)) + time.Duration(rand.Int63n(int64(2*0.2*temp)))
	if backoff < policy.MinWait {
		return policy.MinWait
	}
	if backoff > policy.MaxWait {
		return policy.MaxWait
	}
	return backoff
}

// getRetryPolicy returns the retry policy for the repository using the values present in the spec
// or, if they aren't present, the default values
func getRetryPolicy(clusterRepo *catalog.ClusterRepo) retryPolicy {
	// Default Values for exponentialBackOff function which is used
	// to retry an HTTP call when 429 response code is hit.
	var retryPolicy = retryPolicy{
		MinWait:  1 * time.Second,
		MaxWait:  5 * time.Second,
		MaxRetry: 5,
	}

	if clusterRepo.Spec.ExponentialBackOffValues != nil {
		if clusterRepo.Spec.ExponentialBackOffValues.MaxRetries > 0 {
			retryPolicy.MaxRetry = clusterRepo.Spec.ExponentialBackOffValues.MaxRetries
		}
		if clusterRepo.Spec.ExponentialBackOffValues.MaxWait != "" {
			maxWait, err := time.ParseDuration(clusterRepo.Spec.ExponentialBackOffValues.MaxWait)
			if err == nil {
				retryPolicy.MaxWait = maxWait
			}
		}
		if clusterRepo.Spec.ExponentialBackOffValues.MinWait != "" {
			minWait, err := time.ParseDuration(clusterRepo.Spec.ExponentialBackOffValues.MinWait)
			if err == nil {
				retryPolicy.MinWait = minWait
			}
		}
	}
	return retryPolicy
}

// shouldResetRetries checks to see if the interval has passed or if we need to do a force update
func shouldResetRetries(clusterRepo *catalog.ClusterRepo) bool {
	var lastStatusUpdate time.Time
	for _, field := range clusterRepo.ManagedFields {
		if field.Operation == metav1.ManagedFieldsOperationUpdate && field.Subresource == "status" {
			lastStatusUpdate = field.Time.Time.UTC()
		}
	}
	ociDownloaded := condition.Cond(catalog.OCIDownloaded)
	ociDownloadedTime, _ := time.Parse(time.RFC3339, ociDownloaded.GetLastUpdated(clusterRepo))
	// interval has passed. lastStatusUpdate will always be greater than zero if the ociDownloaded Condition exists
	if !ociDownloadedTime.IsZero() && ociDownloadedTime.Add(interval).Before(timeNow().UTC()) && ociDownloadedTime.Add(interval).After(lastStatusUpdate) {
		return true
	}
	return false
}
