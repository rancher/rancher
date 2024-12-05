package serviceaccounttoken

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rancher/rancher/pkg/features"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// this can't be imported from the impersonation namespace because of a
	// cycle.
	impersonationNamespace string = "cattle-impersonation-system"

	serviceAccountSecretRefIndex string = ".serviceAccountSecretRef"
)

var (
	// How long do we wait before the first secret cleanup run.
	cleanCycleDelay time.Duration = time.Second * 5
	// How often do we clean secrets?
	cleanCycleTime time.Duration = time.Minute * 1

	// How many secrets are fetched at a time?
	// Not all secrets are removed from this list.
	cleaningBatchSize int64 = 1000
)

type secretsCache interface {
	List(namespace string, selector labels.Selector) ([]*corev1.Secret, error)
}

type serviceAccountsCache interface {
	GetByIndex(indexName, key string) ([]*corev1.ServiceAccount, error)
	AddIndexer(indexName string, indexer generic.Indexer[*corev1.ServiceAccount])
}

// StartServiceAccountSecretCleaner starts a background process to cleanup old
// service accounts secrets.
//
// This should only be started in the leader pod.
func StartServiceAccountSecretCleaner(ctx context.Context, secrets secretsCache, serviceAccounts serviceAccountsCache, client clientcorev1.CoreV1Interface) error {
	if !features.CleanStaleSecrets.Enabled() {
		logrus.Info("ServiceAccountSecretCleaner disabled - not starting")
		return nil
	}

	setupServiceAccountsCache(serviceAccounts)

	secretsQueue, err := loadSecretQueue(secrets)
	if err != nil {
		return err
	}

	startTime := time.Now()

	logrus.Infof("Starting ServiceAccountSecretCleaner with %v secrets", secretsQueue.List.Len())
	ticker := time.NewTicker(cleanCycleDelay)

	go func() {
		for {
			select {
			case <-ctx.Done():
				logrus.Info("terminating service account secret cleaner")
				return
			case <-ticker.C:
				if err := CleanServiceAccountSecrets(ctx, client.Secrets(impersonationNamespace), serviceAccounts, secretsQueue.dequeue(cleaningBatchSize)); err != nil {
					logrus.Error(err)
				}
				if l := secretsQueue.List.Len(); l > 0 {
					logrus.Infof("ServiceAccountSecretCleaner has %v secrets remaining", l)
				} else {
					logrus.Infof("ServiceAccountSecretCleaner has no secrets remaining - terminating at %v", time.Since(startTime))
					return
				}
				// This ensures that no matter how long the cleaning takes,
				// we'll always keep the same cycle time.
				ticker.Reset(cleanCycleTime)
			}
		}
	}()

	return nil
}

// CleanServiceAccountSecrets removes unused Secrets for ServiceAccountTokens from a
// namespace in batches.
func CleanServiceAccountSecrets(ctx context.Context, secrets clientv1.SecretInterface, serviceAccounts serviceAccountsCache, secretsToDelete []types.NamespacedName) error {
	var deletionErr error
	var deletedCount int64
	var toBeDeleted []types.NamespacedName

	for i := range secretsToDelete {
		secretRef := secretsToDelete[i]
		serviceAccount, err := findServiceAccountForSecret(ctx, secretRef, serviceAccounts)
		if err != nil {
			deletionErr = errors.Join(deletionErr, err)
			continue
		}

		// If we have a ServiceAccount for this secret, then we can leave it in place.
		if serviceAccount != nil {
			continue
		}

		toBeDeleted = append(toBeDeleted, secretRef)
	}

	for _, secretRef := range toBeDeleted {
		logrus.Debugf("Deleting ServiceAccount Secret %s", secretRef)
		if err := secrets.Delete(ctx, secretRef.Name, metav1.DeleteOptions{}); err != nil {
			deletionErr = errors.Join(deletionErr, err)
		}
		deletedCount++
	}

	if deletedCount > 0 {
		logrus.Infof("SecretCleaner deleted %v secrets", deletedCount)
	}

	return deletionErr
}

func findServiceAccountForSecret(ctx context.Context, secretRef types.NamespacedName, serviceAccounts serviceAccountsCache) (*corev1.ServiceAccount, error) {
	secretAnnotation := secretRef.String()
	serviceAccountList, err := serviceAccounts.GetByIndex(serviceAccountSecretRefIndex, secretAnnotation)
	if err != nil {
		return nil, fmt.Errorf("failed to list service accounts when cleaning: %w", err)
	}

	if len(serviceAccountList) > 0 {
		return serviceAccountList[0], nil
	}

	return nil, nil
}

func labelSelectorForSecrets() (labels.Selector, error) {
	labelReq, err := labels.NewRequirement(ServiceAccountSecretLabel, selection.Exists, nil)
	// This should really never happen...
	if err != nil {
		return nil, fmt.Errorf("failed identifying secrets for cleaning: %s", err)
	}
	labelSelector := labels.NewSelector()
	labelSelector = labelSelector.Add(*labelReq)

	return labelSelector, nil
}

func loadSecretQueue(secrets secretsCache) (*queue[types.NamespacedName], error) {
	selector, err := labelSelectorForSecrets()
	if err != nil {
		return nil, err
	}

	q := newQueue[types.NamespacedName]()
	loaded, err := secrets.List(impersonationNamespace, selector)
	if err != nil {
		return nil, err
	}

	for _, secret := range loaded {
		// Maybe use PartialObjectMetadata?
		q.enqueue(types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace})
	}

	return q, nil
}

// setupServiceAccountsCache must be called early on to setup the correct
// indexing on service accounts.
func setupServiceAccountsCache(cache serviceAccountsCache) {
	cache.AddIndexer(serviceAccountSecretRefIndex, func(s *corev1.ServiceAccount) ([]string, error) {
		if s.ObjectMeta.Annotations == nil {
			return nil, nil
		}
		annotation := s.ObjectMeta.Annotations[ServiceAccountSecretRefAnnotation]
		if annotation != "" {
			return []string{annotation}, nil
		}

		return nil, nil
	})
}
