package k8s

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetSystemSecret(k8sClient *kubernetes.Clientset, secretName string) (*v1.Secret, error) {
	return GetSecret(k8sClient, secretName, metav1.NamespaceSystem)
}

func GetSecret(k8sClient *kubernetes.Clientset, secretName, namespace string) (*v1.Secret, error) {
	return k8sClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
}

func GetSecretsList(k8sClient *kubernetes.Clientset, namespace string) (*v1.SecretList, error) {
	return k8sClient.CoreV1().Secrets("").List(metav1.ListOptions{})
}

func UpdateSecret(k8sClient *kubernetes.Clientset, secret *v1.Secret) error {
	var err error
	_, err = k8sClient.CoreV1().Secrets(secret.Namespace).Update(secret)
	return err
}
