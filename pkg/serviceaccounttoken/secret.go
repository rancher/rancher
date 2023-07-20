package serviceaccounttoken

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const serviceAccountSecretAnnotation = "kubernetes.io/service-account.name"

// secretGetter is an abstraction over any kind of secret getter.
// The caller can use any cache or client it has available, whether that is from norman, wrangler, or client-go,
// as long as it can wrap it in a simplified lambda with this signature.
type secretGetter func(namespace, name string) (*v1.Secret, error)

// EnsureSecretForServiceAccount gets or creates a service account token Secret for the provided Service Account.
// For k8s <1.24, the secret is automatically generated for the service account. For >=1.24, we need to generate it explicitly.
func EnsureSecretForServiceAccount(ctx context.Context, secretGetter secretGetter, clientSet kubernetes.Interface, sa *v1.ServiceAccount) (*v1.Secret, error) {
	if secretGetter == nil {
		secretGetter = func(namespace, name string) (*v1.Secret, error) {
			return clientSet.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		}
	}
	if sa == nil {
		return nil, fmt.Errorf("could not ensure secret for invalid service account")
	}
	secretClient := clientSet.CoreV1().Secrets(sa.Namespace)
	saClient := clientSet.CoreV1().ServiceAccounts(sa.Namespace)
	secretName := ServiceAccountSecretName(sa)
	var secret *v1.Secret
	var err error
	if secretName != "" {
		secret, err = secretGetter(sa.Namespace, secretName)
		if err != nil && !apierror.IsNotFound(err) {
			return nil, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
		}
	}
	if secret == nil || !isSecretForServiceAccount(secret, sa) {
		sc := SecretTemplate(sa)
		secret, err = secretClient.Create(ctx, sc, metav1.CreateOptions{})
		if err != nil {
			return nil, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
		}
		// k8s >=1.24 does not store a reference to the secret, but we need it to refer back to later
		saCopy := sa.DeepCopy()
		saCopy.Secrets = []v1.ObjectReference{{Name: secret.Name}}
		saCopy, err = saClient.Update(ctx, saCopy, metav1.UpdateOptions{})
		if err != nil {
			// clean up the secret we just created
			cleanupErr := secretClient.Delete(ctx, secret.Name, metav1.DeleteOptions{})
			if cleanupErr != nil && !apierror.IsNotFound(cleanupErr) {
				return nil, fmt.Errorf("encountered error while handling service account update error: %v, original error: %w", cleanupErr, err)
			}
			return nil, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
		}
		*sa = *saCopy
	}
	if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
		return secret, nil
	}
	logrus.Infof("EnsureSecretForServiceAccount: waiting for secret [%s] to be populated with token", secret.Name)
	backoff := wait.Backoff{
		Duration: 2 * time.Millisecond,
		Cap:      100 * time.Millisecond,
		Steps:    50,
	}
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		var err error
		// use the secret client, rather than the secret getter, to circumvent the cache
		secret, err = secretClient.Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
		}
		if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error ensuring secret for service account [%s:%s]: %w", sa.Namespace, sa.Name, err)
	}
	return secret, nil
}

// SecretTemplate generate a template of service-account-token Secret for the provided Service Account.
func SecretTemplate(sa *v1.ServiceAccount) *v1.Secret {
	return &v1.Secret{
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
		},
		Type: v1.SecretTypeServiceAccountToken,
	}

}

// serviceAccountSecretPrefix returns the prefix that will be used to generate the secret for the given service account.
func serviceAccountSecretPrefix(sa *v1.ServiceAccount) string {
	return fmt.Sprintf("%s-token-", sa.Name)
}

// ServiceAccountSecretName returns the secret name for the given Service Account.
// If there are more than one, it returns the first.
func ServiceAccountSecretName(sa *v1.ServiceAccount) string {
	if len(sa.Secrets) < 1 {
		return ""
	}
	return sa.Secrets[0].Name
}

func isSecretForServiceAccount(secret *v1.Secret, sa *v1.ServiceAccount) bool {
	if secret.Type != v1.SecretTypeServiceAccountToken {
		return false
	}
	annotations := secret.Annotations
	annotation := annotations[serviceAccountSecretAnnotation]
	return sa.Name == annotation
}
