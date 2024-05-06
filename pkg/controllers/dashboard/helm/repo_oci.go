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

	// The handler is triggered immediately after any changes, including when updating the number of retries done.
	// This check is to prevent the handler from executing before the backoff time has passed
	if !clusterRepo.Status.NextRetryAt.IsZero() && clusterRepo.Status.NextRetryAt.Time.After(timeNow()) {
		return clusterRepo, nil
	}

	retryPolicy := getRetryPolicy(clusterRepo)

	if clusterRepo.Status.NumberOfRetries > retryPolicy.MaxRetry {
		logrus.Infof("Maximum number of retries for oci repository %s reached, will retry after %s", key, interval)
		return clusterRepo, nil
	}

	newStatus := clusterRepo.Status.DeepCopy()

	logrus.Debugf("OCIRepoHandler triggered for clusterrepo %s", clusterRepo.Name)
	var index *repo.IndexFile

	err = ensureIndexConfigMap(clusterRepo, newStatus, o.configMapController)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, newStatus)
	}

	secret, err := catalogv2.GetSecret(o.secretCacheController, &clusterRepo.Spec, clusterRepo.Namespace)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, newStatus)
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
		return o.setErrorCondition(clusterRepo, err, newStatus)
	}
	originalIndexBytes, err := json.Marshal(index)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, newStatus)
	}
	index, err = oci.GenerateIndex(clusterRepo.Spec.URL, secret, clusterRepo.Spec, *newStatus, index)
	// If there is 401 or 403 error code, then we don't reconcile further and wait for 6 hours interval
	var errResp *errcode.ErrorResponse
	if errors.As(err, &errResp) {
		if errResp.StatusCode == http.StatusUnauthorized ||
			errResp.StatusCode == http.StatusForbidden ||
			errResp.StatusCode == http.StatusNotFound {
			return o.set4xxCondition(clusterRepo, errResp, newStatus)
		}

		// If there is 429 error code and max retry is reached, then we don't reconcile further and wait for 6 hours interval,
		// but we also create the configmap for future usecases.
		if errResp.StatusCode == http.StatusTooManyRequests {
			if index != nil && len(index.Entries) > 0 {
				newStatus.URL = clusterRepo.Spec.URL

				index.SortEntries()
				_, err := createOrUpdateMap(clusterRepo.Namespace, index, owner, o.apply)
				if err != nil {
					logrus.Debugf("failed to create/udpate the configmap incase of 4xx statuscode for %s", clusterRepo.Name)
				}
			}

			newStatus.NumberOfRetries++
			if newStatus.NumberOfRetries > retryPolicy.MaxRetry {
				return o.set4xxCondition(clusterRepo, errResp, newStatus)
			}

			backoff := calculateBackoff(clusterRepo, retryPolicy)

			newStatus.NextRetryAt = metav1.Time{Time: timeNow().Add(backoff)}
			clusterRepo.Status = *newStatus
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
		return o.setErrorCondition(clusterRepo, err, newStatus)
	}
	if index == nil || len(index.Entries) <= 0 {
		err = errors.New("there are no helm charts in the repository specified")
		return o.setErrorCondition(clusterRepo, err, newStatus)
	}

	newIndexBytes, err := json.Marshal(index)
	if err != nil {
		return o.setErrorCondition(clusterRepo, err, newStatus)
	}
	// Only update, if the index got updated
	if !bytes.Equal(originalIndexBytes, newIndexBytes) {
		index.SortEntries()
		cm, err := createOrUpdateMap(clusterRepo.Namespace, index, owner, o.apply)
		if err != nil {
			return o.setErrorCondition(clusterRepo, err, newStatus)
		}

		newStatus.URL = clusterRepo.Spec.URL
		newStatus.IndexConfigMapName = cm.Name
		newStatus.IndexConfigMapNamespace = cm.Namespace
		newStatus.IndexConfigMapResourceVersion = cm.ResourceVersion
		newStatus.DownloadTime = downloadTime
	}

	return o.setErrorCondition(clusterRepo, err, newStatus)
}

// setErrorCondition is only called when error happens in the handler, and
// we need to depend on wrangler to reenqueue the handler
func (o *OCIRepohandler) setErrorCondition(clusterRepo *catalog.ClusterRepo, err error, newStatus *catalog.RepoStatus) (*catalog.ClusterRepo, error) {
	var statusErr error
	newStatus.NumberOfRetries = 0
	newStatus.NextRetryAt = metav1.Time{}

	ociDownloaded := condition.Cond(catalog.OCIDownloaded)
	if apierrors.IsConflict(err) {
		ociDownloaded.SetError(newStatus, "", nil)
	} else {
		ociDownloaded.SetError(newStatus, "", err)
	}

	if !equality.Semantic.DeepEqual(newStatus, &clusterRepo.Status) {
		ociDownloaded.LastUpdated(newStatus, time.Now().UTC().Format(time.RFC3339))

		newStatus.ObservedGeneration = clusterRepo.Generation
		clusterRepo.Status = *newStatus
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
func (o *OCIRepohandler) set4xxCondition(clusterRepo *catalog.ClusterRepo, err *errcode.ErrorResponse, newStatus *catalog.RepoStatus) (*catalog.ClusterRepo, error) {
	err.Errors = append(err.Errors, errcode.Error{Message: fmt.Sprintf(" will retry will after %s", interval)})
	ociDownloaded := condition.Cond(catalog.OCIDownloaded)
	if apierrors.IsConflict(err) {
		ociDownloaded.SetError(newStatus, "", nil)
	} else {
		ociDownloaded.SetError(newStatus, "", err)
	}

	if !equality.Semantic.DeepEqual(newStatus, &clusterRepo.Status) {
		// Since status has changed, update the lastUpdatedTime
		ociDownloaded.LastUpdated(newStatus, time.Now().UTC().Format(time.RFC3339))

		newStatus.ObservedGeneration = clusterRepo.Generation
		clusterRepo.Status = *newStatus

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
		if clusterRepo.Spec.ExponentialBackOffValues.MinWait != "" {
			minWait, err := time.ParseDuration(clusterRepo.Spec.ExponentialBackOffValues.MinWait)
			if err != nil {
				logrus.Errorf("Failed to parse minWait, using the default value. err: %s", err)
			}
			if minWait >= 1*time.Second {
				retryPolicy.MinWait = minWait
			} else {
				logrus.Warnf("retry policy minWait should be at least 1 second")
			}
		}
		if clusterRepo.Spec.ExponentialBackOffValues.MaxWait != "" {
			maxWait, err := time.ParseDuration(clusterRepo.Spec.ExponentialBackOffValues.MaxWait)
			if err != nil {
				logrus.Errorf("Failed to parse maxWait, using the default value. err: %s", err)
			}
			if maxWait >= retryPolicy.MinWait {
				retryPolicy.MaxWait = maxWait
			} else {
				logrus.Warnf("retry policy maxWait can not be less than retryPolicy minWait")
				retryPolicy.MaxWait = retryPolicy.MinWait
			}
		}
	}
	return retryPolicy
}

// shouldResetRetries checks to see if the interval has passed or if we need to do a force update
func shouldResetRetries(clusterRepo *catalog.ClusterRepo) bool {
	// check if generation has changed
	if clusterRepo.Generation > clusterRepo.Status.ObservedGeneration {
		return true
	}
	var lastStatusUpdate time.Time
	for _, field := range clusterRepo.ManagedFields {
		// get the last time the status was updated
		if field.Operation == metav1.ManagedFieldsOperationUpdate && field.Subresource == "status" {
			lastStatusUpdate = field.Time.Time.UTC()
		}
	}
	ociDownloaded := condition.Cond(catalog.OCIDownloaded)
	ociDownloadedUpdateTime, _ := time.Parse(time.RFC3339, ociDownloaded.GetLastUpdated(clusterRepo))

	// This condition checks if the following are true:
	// - !ociDownloadedUpdateTime.IsZero() -> If it has finished the first batch of retries at least once
	// - ociDownloadedUpdateTime.Add(interval).Before(timeNow().UTC()) -> If more than the amount of time defined by the interval has passed
	// - ociDownloadedUpdateTime.Add(interval).After(lastStatusUpdate) -> If the handler updated the status (i.e. its retrying) after the interval has passed.
	// The handler will update the status while it's still retrying but won't update the condition until it's finished.
	if !ociDownloadedUpdateTime.IsZero() && ociDownloadedUpdateTime.Add(interval).Before(timeNow().UTC()) && ociDownloadedUpdateTime.Add(interval).After(lastStatusUpdate) {
		return true
	}
	return false
}
