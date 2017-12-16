package k8s

import (
	"bytes"
	"time"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

func UpdateServiceAccountFromYaml(k8sClient *kubernetes.Clientset, serviceAccountYaml string) error {
	serviceAccount := v1.ServiceAccount{}

	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(serviceAccountYaml)))
	err := decoder.Decode(&serviceAccount)
	if err != nil {
		return err
	}
	for retries := 0; retries <= 5; retries++ {
		if _, err = k8sClient.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Create(&serviceAccount); err != nil {
			if apierrors.IsAlreadyExists(err) {
				if _, err = k8sClient.CoreV1().ServiceAccounts(metav1.NamespaceSystem).Update(&serviceAccount); err == nil {
					return nil
				}
			}
		} else {
			return nil
		}
		time.Sleep(time.Second * 5)
	}
	return err
}
