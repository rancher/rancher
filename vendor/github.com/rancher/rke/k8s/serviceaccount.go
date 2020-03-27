package k8s

import (
	"context"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func UpdateServiceAccountFromYaml(k8sClient *kubernetes.Clientset, serviceAccountYaml string) error {
	serviceAccount := v1.ServiceAccount{}

	if err := DecodeYamlResource(&serviceAccount, serviceAccountYaml); err != nil {
		return err
	}
	return retryTo(updateServiceAccount, k8sClient, serviceAccount, DefaultRetries, DefaultSleepSeconds)
}

func updateServiceAccount(k8sClient *kubernetes.Clientset, s interface{}) error {
	serviceAccount := s.(v1.ServiceAccount)
	if _, err := k8sClient.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Create(context.TODO(), &serviceAccount, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Update(context.TODO(), &serviceAccount, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}
