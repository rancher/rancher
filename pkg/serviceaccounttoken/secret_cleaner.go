package serviceaccounttoken

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// this can't be imported from the impersonation namespace because of a
	// cycle.
	impersonationNamespace string = "cattle-impersonation-system"

	pageSize int64 = 100
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

// StartServiceAccountSecretCleaner starts a background process to cleanup old
// service accounts secrets.
//
// This should only be started in the leader pod.
func StartServiceAccountSecretCleaner(ctx context.Context, client clientcorev1.CoreV1Interface) error {
	if strings.ToLower(os.Getenv("DISABLE_SECRET_CLEANER")) == "true" {
		logrus.Info("ServiceAccountSecretCleaner disabled - not starting")
		return nil
	}

	secrets := client.Secrets(impersonationNamespace)
	serviceAccounts := client.ServiceAccounts(impersonationNamespace)

	secretsQueue, err := loadSecretQueue(ctx, secrets, pageSize)
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
				if err := CleanServiceAccountSecrets(ctx, secrets, serviceAccounts, secretsQueue.dequeue(cleaningBatchSize)); err != nil {
					logrus.Error(err)
				}
				if l := secretsQueue.List.Len(); l > 0 {
					logrus.Infof("ServiceAccountSecretCleaner has %v secrets remaining", l)
				} else {
					logrus.Infof("ServiceAccountSecretCleaner has no secrets remaining - terminating at %v", time.Since(startTime))
					return
				}
				// This ensures that no matter how long the cleaning takes,
				// we'll alwas keep the same cycle time.
				ticker.Reset(cleanCycleTime)
			}
		}
	}()

	return nil
}

// CleanServiceAccountSecrets removes unused Secrets for ServiceAccountTokens from a
// namespace in batches.
func CleanServiceAccountSecrets(ctx context.Context, secrets clientv1.SecretInterface, serviceAccounts clientv1.ServiceAccountInterface, secretsToDelete []*corev1.Secret) error {
	var deletionErr error
	var deletedCount int64
	var toBeDeleted []*corev1.Secret

	for i := range secretsToDelete {
		secret := secretsToDelete[i]
		serviceAccount, err := findServiceAccountForSecret(ctx, secret, serviceAccounts)
		if err != nil {
			deletionErr = errors.Join(deletionErr, err)
			continue
		}

		// If have a ServiceAccount for this secret, then we can leave it in
		// place.
		if serviceAccount != nil {
			continue
		}

		toBeDeleted = append(toBeDeleted, secret)
	}

	for _, secret := range toBeDeleted {
		logrus.Debugf("Deleting ServiceAccount Secret %s", logKeyFromObject(secret))
		if err := secrets.Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
			deletionErr = errors.Join(deletionErr, err)
		}
		deletedCount++
	}

	if deletedCount > 0 {
		logrus.Infof("SecretCleaner deleted %v secrets", deletedCount)
	}

	return deletionErr
}

func findServiceAccountForSecret(ctx context.Context, secret *corev1.Secret, serviceAccounts clientv1.ServiceAccountInterface) (*corev1.ServiceAccount, error) {
	serviceAccountList, err := serviceAccounts.List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{
		"authz.cluster.cattle.io/impersonator": "true",
	}).String()})
	if err != nil {
		return nil, fmt.Errorf("failed to list service accounts when cleaning: %w", err)
	}
	secretAnnotation := secret.Namespace + "/" + secret.Name
	for _, sa := range serviceAccountList.Items {
		if sa.Annotations != nil && sa.Annotations[ServiceAccountSecretRefAnnotation] == secretAnnotation {
			return &sa, nil
		}
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

func loadSecretQueue(ctx context.Context, secrets clientv1.SecretInterface, pageSize int64) (*queue[*corev1.Secret], error) {
	selector, err := labelSelectorForSecrets()
	if err != nil {
		return nil, err
	}

	q := newQueue[*corev1.Secret]()
	paginator := newSecretsPaginator(secrets, pageSize, selector)
	for paginator.hasMoreSecrets() {
		secrets, err := paginator.nextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, secret := range secrets {
			// We don't want to store the data for the Secret.
			// Maybe use PartialObjectMetadata?
			stripped := secret.DeepCopy()
			stripped.Data = nil
			q.enqueue(stripped)
		}
	}

	return q, nil
}

func newSecretsPaginator(secrets clientv1.SecretInterface, pageSize int64, selector labels.Selector) *secretsPaginator {
	return &secretsPaginator{
		client:    secrets,
		pageSize:  pageSize,
		firstPage: true,
		selector:  selector,
	}
}

type secretsPaginator struct {
	client   clientv1.SecretInterface
	pageSize int64
	selector labels.Selector

	firstPage     bool
	nextPageToken string
}

// hasMoreSecrets returns true if there are more secrets to be fetched.
func (l *secretsPaginator) hasMoreSecrets() bool {
	return l.firstPage || l.nextPageToken != ""
}

// nextPage returns the next set of namespaces from the iterator.
//
// If no pages remain, an error is returned.
func (l *secretsPaginator) nextPage(ctx context.Context) ([]corev1.Secret, error) {
	if !l.hasMoreSecrets() {
		return nil, errors.New("no more pages")
	}

	options := metav1.ListOptions{
		Limit:         l.pageSize,
		LabelSelector: l.selector.String(),
	}

	if l.nextPageToken != "" {
		options.Continue = l.nextPageToken
	}

	secretsList, err := l.client.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}
	l.firstPage = false
	l.nextPageToken = secretsList.ListMeta.Continue

	return secretsList.Items, nil
}
