package k8s

import (
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetSecret(k8sClient *kubernetes.Clientset, secretName string) (*v1.Secret, error) {
	return k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Get(secretName, metav1.GetOptions{})
}

func UpdateSecret(k8sClient *kubernetes.Clientset, fieldName string, secretData []byte, secretName string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string][]byte{
			fieldName: secretData,
		},
	}
	if _, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Create(secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		// update secret if its already exist
		oldSecret, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Get(secretName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		newData := oldSecret.Data
		newData[fieldName] = secretData
		secret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: metav1.NamespaceSystem,
			},
			Data: newData,
		}
		if _, err := k8sClient.CoreV1().Secrets(metav1.NamespaceSystem).Update(secret); err != nil {
			return err
		}
	}
	return nil
}
