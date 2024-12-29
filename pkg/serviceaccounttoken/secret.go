package serviceaccounttoken

import (
	"context"
	"fmt"
	"strings"
	"time"

	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	// ServiceAccountSecretLabel is the label used to search for the secret belonging to a service account.
	ServiceAccountSecretLabel = "cattle.io/service-account.name"

	// ServiceAccountSecretRefAnnotation is applied to the ServiceAccount to
	// reference the secret that was created.
	ServiceAccountSecretRefAnnotation = "rancher.io/service-account.secret-ref"

	serviceAccountSecretAnnotation = "kubernetes.io/service-account.name"
)

// secretLister is an abstraction over any kind of secret lister.
// The caller can use any cache or client it has available, whether that is from norman, wrangler, or client-go,
// as long as it can wrap it in a simplified lambda with this signature.
type secretLister func(namespace string, selector labels.Selector) ([]*corev1.Secret, error)

// EnsureSecretForServiceAccount gets or creates a service account token Secret for the provided Service Account.
func EnsureSecretForServiceAccount(ctx context.Context, secretsCache corecontrollers.SecretCache, clientSet kubernetes.Interface, sa *corev1.ServiceAccount) (*corev1.Secret, error) {
	if sa == nil {
		return nil, fmt.Errorf("could not ensure secret for invalid service account")
	}
	logrus.Tracef("EnsureSecretForServiceAccount for %s", logKeyFromObject(sa))

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
		return nil, fmt.Errorf("error looking up secret for service account %s: %w", logKeyFromObject(sa), err)
	}

	if secret == nil {
		secret, err = createServiceAccountSecret(ctx, sa, secretLister, secretClient)
		if err != nil {
			return nil, err
		}

		sa, updated, err := annotateSAWithSecret(ctx, sa, secret, clientSet.CoreV1().ServiceAccounts(sa.Namespace), secretClient)
		if err != nil {
			return nil, err
		}

		if updated {
			secretRef, err := secretRefFromSA(sa)
			if err != nil {
				return nil, err
			}

			secret, err = clientSet.CoreV1().Secrets(secretRef.Namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("reloading referenced secret for SA %s: %w", logKeyFromObject(sa), err)
			}
		}
	}

	if len(secret.Data[corev1.ServiceAccountTokenKey]) > 0 {
		return secret, nil
	}

	logrus.Infof("EnsureSecretForServiceAccount: waiting for secret %s for service account %s to be populated with token", logKeyFromObject(secret), logKeyFromObject(sa))
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
			return false, fmt.Errorf("error ensuring secret for service account %s: %w", logKeyFromObject(sa), err)
		}
		if len(secret.Data[corev1.ServiceAccountTokenKey]) > 0 {
			logrus.Infof("EnsureSecretForServiceAccount: got the service account token for service account %s in %s", logKeyFromObject(sa), time.Now().Sub(start))
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
// Can return a nil secret and a nil error if no secret is found
func ServiceAccountSecret(ctx context.Context, sa *corev1.ServiceAccount, secretLister secretLister, secretClient clientv1.SecretInterface) (*corev1.Secret, error) {
	if sa == nil {
		return nil, fmt.Errorf("cannot get secret for nil service account")
	}

	annotations := sa.Annotations
	if annotations == nil {
		annotations = map[string]string{}
	}

	secretAnn := annotations[ServiceAccountSecretRefAnnotation]
	if secretAnn != "" {
		secret, err := secretFromSA(ctx, sa, secretClient)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		// If secret is nil drops down to find by listing secrets.
		if secret != nil {
			return secret, nil
		}
		logrus.Infof("ServiceAccount secret did not exist for ServiceAccount %s falling back to listing mechanism", sa.Name)
	}

	return findSecretForSA(ctx, sa, secretLister, secretClient)
}

func findSecretForSA(ctx context.Context, sa *corev1.ServiceAccount, secretLister secretLister, secretClient clientv1.SecretInterface) (*corev1.Secret, error) {
	secrets, err := secretLister(sa.Namespace, labels.SelectorFromSet(map[string]string{
		ServiceAccountSecretLabel: sa.Name,
	}))
	if err != nil {
		return nil, fmt.Errorf("could not get secrets for service account %s: %w", sa.Name, err)
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

		logrus.Warnf("EnsureSecretForServiceAccount: secret %s is invalid for service account [%s], deleting", logKeyFromObject(s), sa.Name)
		err = secretClient.Delete(ctx, s.Name, metav1.DeleteOptions{})
		if err != nil {
			// we don't want to return the delete failure since the success/failure of the cleanup shouldn't affect
			// the ability of the caller to use any identified, valid secret
			logrus.Errorf("unable to delete secret %s: %v", logKeyFromObject(s), err)
		}
	}

	return result, nil
}

func secretFromSA(ctx context.Context, sa *corev1.ServiceAccount, secretClient clientv1.SecretInterface) (*corev1.Secret, error) {
	secretRef, err := secretRefFromSA(sa)
	if err != nil {
		return nil, err
	}

	secret, err := secretClient.Get(ctx, secretRef.Name, metav1.GetOptions{})
	if err != nil {
		// During the encryption key update this can fail for deleted secrets.
		// This manifests as an Internal Error from K8s.
		// In this case, we can ignore it and let a secret be created.
		if !apierrors.IsInternalError(err) {
			return nil, fmt.Errorf("could not get secret for service account: %w", err)
		}
		logrus.Infof("internal error when getting a secret %s - not using the secret", secret.Name)
		return nil, nil
	}

	return secret, nil
}

func isSecretForServiceAccount(secret *corev1.Secret, sa *corev1.ServiceAccount) bool {
	if secret.Type != corev1.SecretTypeServiceAccountToken {
		return false
	}
	annotations := secret.Annotations
	annotation := annotations[serviceAccountSecretAnnotation]

	return sa.Name == annotation
}

func createServiceAccountSecret(ctx context.Context, sa *corev1.ServiceAccount, secretLister secretLister, secretClient clientv1.SecretInterface) (*corev1.Secret, error) {
	sc := SecretTemplate(sa)
	secret, err := secretClient.Create(ctx, sc, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error ensuring secret for service account %s: %w", logKeyFromObject(sa), err)
	}

	return secret, nil
}

// returns true if the secret has changed.
func annotateSAWithSecret(ctx context.Context, sa *corev1.ServiceAccount, secret *corev1.Secret, saClient clientv1.ServiceAccountInterface, secretClient clientv1.SecretInterface) (*corev1.ServiceAccount, bool, error) {
	sa = sa.DeepCopy()

	if sa.Annotations == nil {
		sa.Annotations = map[string]string{}
	}
	secretAnnotation := secret.Namespace + "/" + secret.Name

	// If the SA is already annotated with the secret already
	if ann := sa.Annotations[ServiceAccountSecretRefAnnotation]; ann == secretAnnotation {
		return sa, false, nil
	}

	sa.Annotations[ServiceAccountSecretRefAnnotation] = secretAnnotation

	updated, err := saClient.Update(ctx, sa, metav1.UpdateOptions{})
	if err == nil {
		return updated, false, nil
	}
	if !apierrors.IsConflict(err) {
		logrus.Debugf("Failed to update ServiceAccount for %s: %s", logKeyFromObject(sa), err)
		return nil, false, err
	}

	// Rollback the optimistically created secret
	logrus.Infof("Rolling back ServiceAccount secret for %s", logKeyFromObject(secret))
	if err := secretClient.Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
		return nil, false, fmt.Errorf("deleting optimistically locked secret for %s - %s: %w", logKeyFromObject(secret), logKeyFromObject(sa), err)
	}
	// Load the version that triggered the issue
	logrus.Debugf("Reloading ServiceAccount %s", logKeyFromObject(sa))
	updated, err = saClient.Get(ctx, sa.Name, metav1.GetOptions{})
	if err != nil {
		return nil, false, fmt.Errorf("getting updated service account %s: %w", logKeyFromObject(sa), err)
	}

	return updated, true, nil

}

func logKeyFromObject(obj metav1.Object) string {
	return fmt.Sprintf("[%s:%s]", obj.GetNamespace(), obj.GetName())
}

func secretRefFromSA(sa *corev1.ServiceAccount) (*types.NamespacedName, error) {
	if sa.Annotations == nil {
		return nil, nil
	}
	if ann := sa.Annotations[ServiceAccountSecretRefAnnotation]; ann != "" {
		elements := strings.Split(ann, "/")
		if len(elements) != 2 {
			return nil, fmt.Errorf("too many elements parsing ServiceAccount secret reference: %s", ann)
		}
		return &types.NamespacedName{Namespace: elements[0], Name: elements[1]}, nil
	}

	return nil, nil
}
