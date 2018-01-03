package k8s

import (
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

func UpdatePodSecurityPolicyFromYaml(k8sClient *kubernetes.Clientset, pspYaml string) error {
	psp := v1beta1.PodSecurityPolicy{}
	if err := decodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}
	return retryTo(updatePodSecurityPolicy, k8sClient, psp, DefaultRetries, DefaultSleepSeconds)
}

func updatePodSecurityPolicy(k8sClient *kubernetes.Clientset, p interface{}) error {
	psp := p.(v1beta1.PodSecurityPolicy)
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
