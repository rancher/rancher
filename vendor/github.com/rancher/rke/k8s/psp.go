package k8s

import (
	"time"

	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

func UpdatePodSecurityPolicyFromYaml(k8sClient *kubernetes.Clientset, pspYaml string) error {
	psp := v1beta1.PodSecurityPolicy{}
	err := decodeYamlResource(&psp, pspYaml)
	if err != nil {
		return err
	}
	for retries := 0; retries <= 5; retries++ {
		if err = updatePodSecurityPolicy(k8sClient, psp); err != nil {
			time.Sleep(time.Second * 5)
			continue
		}
		return nil
	}
	return err
}

func updatePodSecurityPolicy(k8sClient *kubernetes.Clientset, psp v1beta1.PodSecurityPolicy) error {
	if _, err := k8sClient.ExtensionsV1beta1().PodSecurityPolicies().Create(&psp); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.ExtensionsV1beta1().PodSecurityPolicies().Update(&psp); err != nil {
			return err
		}
	}
	return nil

}
