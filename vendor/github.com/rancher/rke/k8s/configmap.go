package k8s

import (
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func UpdateConfigMap(k8sClient *kubernetes.Clientset, configYaml []byte, configMapName string) error {
	cfgMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			configMapName: string(configYaml),
		},
	}

	if _, err := k8sClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Create(cfgMap); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Update(cfgMap); err != nil {
			return err
		}
	}
	return nil
}

func GetConfigMap(k8sClient *kubernetes.Clientset, configMapName string) (*v1.ConfigMap, error) {
	return k8sClient.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(configMapName, metav1.GetOptions{})
}
