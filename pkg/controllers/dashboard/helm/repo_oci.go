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
	"github.com/rancher/rancher/pkg/catalogv2/oci/capturewindowclient"
	catalogcontrollers "github.com/rancher/rancher/pkg/generated/controllers/catalog.cattle.io/v1"
	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/wrangler/v3/pkg/apply"
	corev1controllers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

var (
	timeNow               = time.Now
	defaultOCIRetryPolicy = retryPolicy{
		MinWait:  1 * time.Second,
		MaxWait:  5 * time.Second,
		MaxRetry: 5,
	}
)

const (
	defaultOCIInterval = 24 * time.Hour
	ociCondition       = catalog.OCIDownloaded
)

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

// This handler is triggered in the following cases
// * When the spec of the ClusterRepo is changed.
// * When there is no error from the handler, at a regular interval of 24 hours.
// * When there is an error from the handler, at the wrangler's default error interval.
// * When the response from OCI registry is anything 4xx HTTP status code, at an interval of 24 hours or the duration time to wait which is calculated by the backoff function.
func (o *OCIRepohandler) onClusterRepoChange(key string, clusterRepo *catalog.ClusterRepo) (*catalog.ClusterRepo, error) {
	if clusterRepo == nil {
		return nil, nil
	}
	// Ignore non OCI ClusterRepos
	if !registry.IsOCI(clusterRepo.Spec.URL) {
		return clusterRepo, nil
	}

	ociInterval := defaultOCIInterval

	newStatus := clusterRepo.Status.DeepCopy()
	retryPolicy, err := getRetryPolicy(clusterRepo)
	if err != nil {
		err = fmt.Errorf("failed to get retry policy: %w", err)
		return setErrorCondition(clusterRepo, err, newStatus, ociInterval, ociCondition, o.clusterRepoController)
	}

	err = ensureIndexConfigMap(clusterRepo, newStatus, o.configMapController)
	if err != nil {
		err = fmt.Errorf("failed to ensure index configmap: %w", err)
		return setErrorCondition(clusterRepo, err, newStatus, ociInterval, ociCondition, o.clusterRepoController)
	}

	if shouldSkip(clusterRepo, retryPolicy, ociCondition, ociInterval, o.clusterRepoController, newStatus) {
		return clusterRepo, nil
	}
	newStatus.ShouldNotSkip = false

	logrus.Debugf("OCIRepoHandler triggered for clusterrepo %s", clusterRepo.Name)
	var index *repo.IndexFile

	secret, err := catalogv2.GetSecret(o.secretCacheController, &clusterRepo.Spec, clusterRepo.Namespace)
	if err != nil {
		logrus.Errorf("Error while fetching secret for cluster repo %s: %v", clusterRepo.Name, err)
		reason := apierrors.ReasonForError(err)
		if reason != metav1.StatusReasonUnknown {
			err = fmt.Errorf("failed to fetch secret: %s", reason)
		}
		return setErrorCondition(clusterRepo, err, newStatus, ociInterval, ociCondition, o.clusterRepoController)
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
		return setErrorCondition(clusterRepo, fmt.Errorf("error while getting indexfile: %w", err), newStatus, ociInterval, ociCondition, o.clusterRepoController)
	}
	originalIndexBytes, err := json.Marshal(index)
	if err != nil {
		logrus.Errorf("Error while marshalling indexfile for cluster repo %s: %v", clusterRepo.Name, err)
		return setErrorCondition(clusterRepo, fmt.Errorf("error while reading indexfile"), newStatus, ociInterval, ociCondition, o.clusterRepoController)
	}
	// Create a new oci client
	ociClient, err := oci.NewClient(clusterRepo.Spec.URL, clusterRepo.Spec, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to create an OCI client for url %s: %w", clusterRepo.Spec.URL, err)
	}

	index, err = oci.GenerateIndex(ociClient, clusterRepo.Spec.URL, secret, clusterRepo.Spec, *newStatus, index)
	var errResp *errcode.ErrorResponse
	// If there is 401/403/404 error code, then we don't reconcile further at all and wait for user to fix the issue.
	if errors.As(err, &errResp) && (errResp.StatusCode == http.StatusUnauthorized ||
		errResp.StatusCode == http.StatusForbidden ||
		errResp.StatusCode == http.StatusNotFound) {
		errorMsg := fmt.Sprintf("error %d: %s", errResp.StatusCode, http.StatusText(errResp.StatusCode))
		newStatus.NumberOfRetries = 0
		newStatus.NextRetryAt = metav1.Time{}

		return setCondition(clusterRepo, err, newStatus, ociCondition, errorMsg, o.clusterRepoController)
	}
	// If there is 429 error code and max retry is reached, then we don't reconcile further and wait for 24 hours interval,
	// but we also create the configmap for future usecases.
	if errors.As(err, &errResp) && errResp.StatusCode == http.StatusTooManyRequests {
		if index != nil && len(index.Entries) > 0 {
			newStatus.URL = clusterRepo.Spec.URL

			index.SortEntries()
			_, err := createOrUpdateMap(clusterRepo.Namespace, index, owner, o.apply)
			if err != nil {
				logrus.Debugf("failed to create/udpate the configmap incase of 4xx statuscode for %s", clusterRepo.Name)
			}
		}

		// If OCIRegistryBackOffDuration is already defined by the registry,
		// we ignore the exponentialBackOffValues set by the user.
		var backoff time.Duration
		ociRegistryBackOffDuration := ociClient.HTTPClient.Transport.(*capturewindowclient.Transport).BackOffDuration
		if ociRegistryBackOffDuration > 0.0 {
			backoff = time.Duration(ociRegistryBackOffDuration) * time.Second
			ociClient.HTTPClient.Transport.(*capturewindowclient.Transport).BackOffDuration = 0.0
			newStatus.ShouldNotSkip = true
		} else {
			backoff = calculateBackoff(clusterRepo, retryPolicy)

			newStatus.NumberOfRetries++
			if newStatus.NumberOfRetries > retryPolicy.MaxRetry {
				newStatus.NumberOfRetries = 0
				newStatus.NextRetryAt = metav1.Time{}
				return o.setConditionWithInterval(clusterRepo, errResp, newStatus, nil, ociInterval)
			}
		}

		newStatus.NextRetryAt = metav1.Time{Time: timeNow().UTC().Add(backoff)}
		return o.setConditionWithInterval(clusterRepo, errResp, newStatus, &backoff, ociInterval)

	}
	// If there is an error, we wait for the next interval to happen.
	if err != nil {
		return o.setConditionWithInterval(clusterRepo, err, newStatus, nil, ociInterval)
	}
	if index == nil || len(index.Entries) <= 0 {
		err = errors.New("there are no helm charts in the repository specified")
		return setErrorCondition(clusterRepo, err, newStatus, ociInterval, ociCondition, o.clusterRepoController)
	}

	newIndexBytes, err := json.Marshal(index)
	if err != nil {
		logrus.Errorf("Error while marshalling indexfile for cluster repo %s: %v", clusterRepo.Name, err)
		return setErrorCondition(clusterRepo, fmt.Errorf("error while reading indexfile"), newStatus, ociInterval, ociCondition, o.clusterRepoController)
	}
	// Only update, if the index got updated
	if !bytes.Equal(originalIndexBytes, newIndexBytes) {
		index.SortEntries()
		cm, err := createOrUpdateMap(clusterRepo.Namespace, index, owner, o.apply)
		if err != nil {
			return setErrorCondition(clusterRepo, fmt.Errorf("error while creating or updating confimap"), newStatus, ociInterval, ociCondition, o.clusterRepoController)
		}

		newStatus.URL = clusterRepo.Spec.URL
		newStatus.IndexConfigMapName = cm.Name
		newStatus.IndexConfigMapNamespace = cm.Namespace
		newStatus.IndexConfigMapResourceVersion = cm.ResourceVersion
		newStatus.DownloadTime = downloadTime
	}

	return setErrorCondition(clusterRepo, nil, newStatus, ociInterval, ociCondition, o.clusterRepoController)
}

// setConditionWithInterval is called to reenqueue the object
// after the interval of 24 hours.
func (o *OCIRepohandler) setConditionWithInterval(clusterRepo *catalog.ClusterRepo, err error, newStatus *catalog.RepoStatus, backoff *time.Duration, ociInterval time.Duration) (*catalog.ClusterRepo, error) {
	var errResp *errcode.ErrorResponse
	var errorMsg string
	if errors.As(err, &errResp) {
		errorMsg = fmt.Sprintf("error %d: %s", errResp.StatusCode, http.StatusText(errResp.StatusCode))
		if backoff != nil {
			errorMsg = fmt.Sprintf("%s. %s", errorMsg, fmt.Sprintf("Will retry after %s", backoff.Round(time.Second)))
		} else {
			errorMsg = fmt.Sprintf("%s. %s", errorMsg, fmt.Sprintf("Will retry after %s", ociInterval.Round(time.Second)))
		}
	} else {
		errorMsg = err.Error()
	}
	backoffInterval := ociInterval.Round(time.Second)
	if backoff != nil {
		backoffInterval = backoff.Round(time.Second)
	}
	return setConditionWithInterval(clusterRepo, err, newStatus, backoffInterval, ociCondition, errorMsg, o.clusterRepoController)
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
		logrus.Errorf("failed to create reader for index file for URL %s: %v", clusterRepoSpec.URL, err)
		return indexFile, fmt.Errorf("failed to read indexfile for cluster repo")
	}
	defer gz.Close()
	data, err = io.ReadAll(gz)
	if err != nil {
		logrus.Errorf("failed to read index file for URL %s: %v", clusterRepoSpec.URL, err)
		return indexFile, fmt.Errorf("failed to read indexfile for cluster repo")
	}
	if err := json.Unmarshal(data, indexFile); err != nil {
		logrus.Errorf("failed to unmarshal index file for URL %s: %v", clusterRepoSpec.URL, err)
		return indexFile, fmt.Errorf("failed to unmarshal indexfile for cluster repo")
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
func getRetryPolicy(clusterRepo *catalog.ClusterRepo) (retryPolicy, error) {
	// Default Values for exponentialBackOff function which is used
	// to retry an HTTP call when 429 response code is hit.

	policy := defaultOCIRetryPolicy
	if !registry.IsOCI(clusterRepo.Spec.URL) {
		policy = defaultHandlerErrRetryPolicy
	}

	if clusterRepo.Spec.ExponentialBackOffValues != nil {
		// Set MaxRetry if specified and valid
		if clusterRepo.Spec.ExponentialBackOffValues.MaxRetries > 0 {
			policy.MaxRetry = clusterRepo.Spec.ExponentialBackOffValues.MaxRetries
		}

		// Set MinWait if specified and valid
		if clusterRepo.Spec.ExponentialBackOffValues.MinWait >= 1 {
			policy.MinWait = time.Duration(clusterRepo.Spec.ExponentialBackOffValues.MinWait) * time.Second
		} else if clusterRepo.Spec.ExponentialBackOffValues.MinWait != 0 {
			return policy, errors.New("minWait must be at least 1 second")
		}

		// Set MaxWait if specified and valid
		if clusterRepo.Spec.ExponentialBackOffValues.MaxWait > 0 {
			policy.MaxWait = time.Duration(clusterRepo.Spec.ExponentialBackOffValues.MaxWait) * time.Second
		}
	}

	// Ensure MaxWait is not less than MinWait
	if policy.MaxWait < policy.MinWait {
		return policy, errors.New("maxWait must be greater than or equal to minWait")
	}

	return policy, nil
}
