package common

import (
	"fmt"
	"reflect"
	"strings"

	corev1 "github.com/rancher/types/apis/core/v1"
	"github.com/rancher/types/namespace"
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

func GetName(configType string, field string) string {
	return fmt.Sprintf("%s:%s-%s", SecretsNamespace, strings.ToLower(configType), field)
}
