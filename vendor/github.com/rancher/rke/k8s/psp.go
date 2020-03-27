package k8s

import (
	"context"

	"k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func UpdatePodSecurityPolicyFromYaml(k8sClient *kubernetes.Clientset, pspYaml string) error {
	psp := v1beta1.PodSecurityPolicy{}
	if err := DecodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}
	return retryTo(updatePodSecurityPolicy, k8sClient, psp, DefaultRetries, DefaultSleepSeconds)
}

func updatePodSecurityPolicy(k8sClient *kubernetes.Clientset, p interface{}) error {
	psp := p.(v1beta1.PodSecurityPolicy)
	if _, err := k8sClient.PolicyV1beta1().PodSecurityPolicies().Create(context.TODO(), &psp, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := k8sClient.PolicyV1beta1().PodSecurityPolicies().Update(context.TODO(), &psp, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil

}
