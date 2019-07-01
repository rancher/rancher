package k8s

import (
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func DeletePods(k8sClient *kubernetes.Clientset, podList *v1.PodList) error {
	for _, pod := range podList.Items {
		if err := k8sClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func ListPodsByLabel(k8sClient *kubernetes.Clientset, label string) (*v1.PodList, error) {
	pods, err := k8sClient.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	return pods, nil
}
