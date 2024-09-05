package serviceaccounttoken

import (
	"context"
	"fmt"
	"sync"
	"time"

	corecontrollers "github.com/rancher/wrangler/v2/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// ServiceAccountSecretLabel is the label used to search for the secret belonging to a service account.
	ServiceAccountSecretLabel = "cattle.io/service-account.name"

	serviceAccountSecretAnnotation = "kubernetes.io/service-account.name"
)

// secretLister is an abstraction over any kind of secret lister.
// The caller can use any cache or client it has available, whether that is from norman, wrangler, or client-go,
// as long as it can wrap it in a simplified lambda with this signature.
type secretLister func(namespace string, selector labels.Selector) ([]*corev1.Secret, error)

var lockMap sync.Map

func getLock(key string) *sync.Mutex {
	actual, _ := lockMap.LoadOrStore(key, &sync.Mutex{})
	return actual.(*sync.Mutex)
}

// EnsureSecretForServiceAccount gets or creates a service account token Secret for the provided Service Account.
//
// If lockPrefix is provided, this is used as a prefix for the lock to provide
// context for concurrent requests. No concurrent creates will be allowed for a
// service account.
//
// The lockPrefix should be of the same format as a generateName e.g. cluster-
//
// For example, if this is "cluster-1" and the ServiceAccount is
// "default:test-sa" subsequent requests to create a ServiceAccount with the
// same name in "cluster-1" will be blocked until the first request has
// completed.
//
// Note: This mutex is per-replica, currently there's no attempt to co-ordinate
// across replicas.
func EnsureSecretForServiceAccount(ctx context.Context, secretsCache corecontrollers.SecretCache, clientSet kubernetes.Interface, sa *corev1.ServiceAccount, lockPrefix string) (*corev1.Secret, error) {
	if sa == nil {
		return nil, fmt.Errorf("could not ensure secret for invalid service account")
	}
	logrus.Tracef("EnsureSecretForServiceAccount for %s:%s", sa.Namespace, sa.Name)

	lockKey := fmt.Sprintf("%v%v-%v", lockPrefix, sa.Namespace, sa.Name)
	secretClient := clientSet.CoreV1().Secrets(sa.Namespace)
	var secretLister secretLister
	if secretsCache != nil {
		secretLister = secretsCache.List
	} else {
		secretLister = func(_ string, selector labels.Selector) ([]*corev1.Secret, error) {
			secretList, err := secretClient.List(ctx, metav1.ListOptions{
				LabelSelector: selector.String(),
			})
			if err != nil {
				return nil, err
			}
			result := make([]*corev1.Secret, len(secretList.Items))
			for i := range secretList.Items {
				result[i] = &secretList.Items[i]
			}
			return result, nil
		}
	}

	secret, err := ServiceAccountSecret(ctx, sa, secretLister, secretClient)
	if err != nil {
		return nil, fmt.Errorf("error looking up secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
	}

	if secret == nil {
		secret, err = createServiceAccountSecret(ctx, sa, secretLister, secretClient, lockKey)
		if err != nil {
			return nil, err
		}
	}

	if len(secret.Data[corev1.ServiceAccountTokenKey]) > 0 {
		return secret, nil
	}

	logrus.Infof("EnsureSecretForServiceAccount: waiting for secret [%s:%s] for service account [%s:%s] to be populated with token", secret.Namespace, secret.Name, sa.Namespace, sa.Name)
	backoff := wait.Backoff{
		Duration: 2 * time.Millisecond,
		Cap:      100 * time.Millisecond,
		Steps:    50,
	}
	start := time.Now()
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		logrus.Tracef("Waiting for the secret with backoff for %s/%s", sa.GetNamespace(), sa.GetName())
		var err error
		// use the secret client, rather than the secret getter, to circumvent the cache
		secret, err = secretClient.Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
		}
		if len(secret.Data[corev1.ServiceAccountTokenKey]) > 0 {
			logrus.Infof("EnsureSecretForServiceAccount: got the service account token for service account [%s:%s] in %s %s ", sa.GetNamespace(), sa.GetName(), time.Now().Sub(start), lockPrefix)
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, err // err is already wapped inside the Wait.
	}

	return secret, nil
}

// SecretTemplate generate a template of service-account-token Secret for the provided Service Account.
func SecretTemplate(sa *corev1.ServiceAccount) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: serviceAccountSecretPrefix(sa),
			Namespace:    sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Name:       sa.Name,
					UID:        sa.UID,
				},
			},
			Annotations: map[string]string{
				serviceAccountSecretAnnotation: sa.Name,
			},
			Labels: map[string]string{
				ServiceAccountSecretLabel: sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

}

// serviceAccountSecretPrefix returns the prefix that will be used to generate the secret for the given service account.
func serviceAccountSecretPrefix(sa *corev1.ServiceAccount) string {
	return fmt.Sprintf("%s-token-", sa.Name)
}

// ServiceAccountSecret returns the secret for the given Service Account.
// If there are more than one, it returns the first. Can return a nil secret
// and a nil error if no secret is found
func ServiceAccountSecret(ctx context.Context, sa *corev1.ServiceAccount, secretLister secretLister, secretClient clientv1.SecretInterface) (*corev1.Secret, error) {
	if sa == nil {
		return nil, fmt.Errorf("cannot get secret for nil service account")
	}

	secrets, err := secretLister(sa.Namespace, labels.SelectorFromSet(map[string]string{
		ServiceAccountSecretLabel: sa.Name,
	}))
	if err != nil {
		return nil, fmt.Errorf("could not get secrets for service account: %w", err)
	}

	if len(secrets) < 1 {
		return nil, nil
	}

	var result *corev1.Secret
	// There is an issue here  - multiple calls could result in multiple attempts
	// to delete secrets while the secret deletion is ongoing.
	for _, s := range secrets {
		if isSecretForServiceAccount(s, sa) {
			if result == nil {
				result = s
			}
			continue
		}

		logrus.Warnf("EnsureSecretForServiceAccount: secret [%s:%s] is invalid for service account [%s], deleting", s.Namespace, s.Name, sa.Name)
		err = secretClient.Delete(ctx, s.Name, metav1.DeleteOptions{})
		if err != nil {
			// we don't want to return the delete failure since the success/failure of the cleanup shouldn't affect
			// the ability of the caller to use any identified, valid secret
			logrus.Errorf("unable to delete secret [%s:%s]: %v", s.Namespace, s.Name, err)
		}
	}

	return result, nil
}

func isSecretForServiceAccount(secret *corev1.Secret, sa *corev1.ServiceAccount) bool {
	if secret.Type != corev1.SecretTypeServiceAccountToken {
		return false
	}
	annotations := secret.Annotations
	annotation := annotations[serviceAccountSecretAnnotation]

	return sa.Name == annotation
}

func createServiceAccountSecret(ctx context.Context, sa *corev1.ServiceAccount, secretLister secretLister, secretClient clientv1.SecretInterface, lockKey string) (*corev1.Secret, error) {
	mutex := getLock(lockKey)
	mutex.Lock()
	defer func(key string) {
		mutex.Unlock()
		lockMap.Delete(lockKey)
	}(lockKey)

	// We could have been waiting for the Mutex to unlock in a parallel run of
	// createServiceAccountSecret - check again for the secret existing.
	secret, err := ServiceAccountSecret(ctx, sa, secretLister, secretClient)
	if err != nil {
		return nil, err
	}
	if secret != nil {
		return secret, nil
	}

	sc := SecretTemplate(sa)
	secret, err = secretClient.Create(ctx, sc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
	}

	return secret, nil
}
