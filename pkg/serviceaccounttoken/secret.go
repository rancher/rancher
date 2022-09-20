package serviceaccounttoken

import (
	"context"
	"time"

	"github.com/rancher/wrangler/pkg/name"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateSecretForServiceAccount creates a service-account-token Secret for the provided Service Account.
// If the secret already exists, the existing one is returned.
func CreateSecretForServiceAccount(ctx context.Context, clientSet kubernetes.Interface, sa *v1.ServiceAccount) (*v1.Secret, error) {
	secretName := ServiceAccountSecretName(sa)
	secretClient := clientSet.CoreV1().Secrets(sa.Namespace)
	secret, err := secretClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if !apierror.IsNotFound(err) {
			return nil, err
		}
		sc := SecretTemplate(sa)
		secret, err = secretClient.Create(ctx, sc, metav1.CreateOptions{})
		if err != nil {
			if !apierror.IsAlreadyExists(err) {
				return nil, err
			}
			secret, err = secretClient.Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
		}
	}
	if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
		return secret, nil
	}
	logrus.Infof("createSecretForServiceAccount: waiting for secret [%s] to be populated with token", secretName)
	for {
		if len(secret.Data[v1.ServiceAccountTokenKey]) > 0 {
			return secret, nil
		}
		time.Sleep(2 * time.Second)
		secret, err = secretClient.Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
}

// SecretTemplate generate a template of service-account-token Secret for the provided Service Account.
func SecretTemplate(sa *v1.ServiceAccount) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountSecretName(sa),
			Namespace: sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "ServiceAccount",
					Name:       sa.Name,
					UID:        sa.UID,
				},
			},
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": sa.Name,
			},
		},
		Type: v1.SecretTypeServiceAccountToken,
	}

}

// ServiceAccountSecretName returns the secret name for the given Service Account.
func ServiceAccountSecretName(sa *v1.ServiceAccount) string {
	return name.SafeConcatName(sa.Name, "token")
}
