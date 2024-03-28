package common

import (
	"fmt"
	"reflect"
	"strings"

	corev1 "github.com/rancher/rancher/pkg/generated/norman/core/v1"
	"github.com/rancher/rancher/pkg/namespace"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SecretsNamespace = namespace.GlobalNamespace

func CreateOrUpdateSecrets(secrets corev1.SecretInterface, secretInfo string, field string, authType string) error {
	if secretInfo == "" {
		return nil
	}
	name := fmt.Sprintf("%s-%s", authType, field)
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: SecretsNamespace,
		},
		StringData: map[string]string{field: secretInfo},
		Type:       v1.SecretTypeOpaque,
	}

	curr, err := secrets.Controller().Lister().Get(SecretsNamespace, name)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting secret for %s : %v", name, err)
	}
	if err == nil && !reflect.DeepEqual(curr.Data, secret.Data) {
		_, err = secrets.Update(secret)
		if err != nil {
			return fmt.Errorf("error updating secret %s: %v", name, err)
		}
	} else if apierrors.IsNotFound(err) {
		_, err = secrets.Create(secret)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("error creating secret %s %v", name, err)
		}
	}
	return nil
}

func ReadFromSecret(secrets corev1.SecretInterface, secretInfo string, field string) (string, error) {
	if strings.HasPrefix(secretInfo, SecretsNamespace) {
		data, err := ReadFromSecretData(secrets, secretInfo)
		if err != nil {
			return "", err
		}
		for key, val := range data {
			if key == field {
				return string(val), nil
			}
		}
	}
	return secretInfo, nil
}

func ReadFromSecretData(secrets corev1.SecretInterface, secretInfo string) (map[string][]byte, error) {
	if strings.HasPrefix(secretInfo, SecretsNamespace) {
		split := strings.SplitN(secretInfo, ":", 2)
		if len(split) == 2 {
			secret, err := secrets.GetNamespaced(split[0], split[1], metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("error getting secret %s %v", secretInfo, err)
			}
			return secret.Data, nil
		}
	}
	return nil, nil
}

// GetFullSecretName returns a formatted name for a secret associated with an auth provider,
// given a config type and its field.
func GetFullSecretName(configType string, field string) string {
	return fmt.Sprintf("%s:%s-%s", SecretsNamespace, strings.ToLower(configType), field)
}

// DeleteSecret deletes a secret associated with an auth provider.
func DeleteSecret(secrets corev1.SecretInterface, configType string, field string) error {
	secretName := fmt.Sprintf("%s-%s", strings.ToLower(configType), strings.ToLower(field))
	return secrets.DeleteNamespaced(SecretsNamespace, secretName, &metav1.DeleteOptions{})
}

// SavePasswordSecret creates a secret out of a password, config type, and field name.
func SavePasswordSecret(secrets corev1.SecretInterface, password string, fieldName string, authType string) (string, error) {
	if err := CreateOrUpdateSecrets(secrets, password, strings.ToLower(fieldName), strings.ToLower(authType)); err != nil {
		return "", err
	}
	return GetFullSecretName(authType, strings.ToLower(fieldName)), nil
}
