package registries

import (
	"strings"

	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	corev1 "k8s.io/api/core/v1"
)

func CheckAllClusterPodsForRegistryPrefix(podsList *v1.SteveCollection, substring string) (bool, error) {

	for _, pod := range podsList.Data {
		podSpec := &corev1.PodSpec{}
		err := v1.ConvertToK8sType(pod.Spec, podSpec)
		if err != nil {
			return false, err
		}
		for _, container := range podSpec.Containers {
			if !strings.Contains(container.Image, substring) {
				return false, nil
			}
		}
	}
	return true, nil
}

func CheckAllClusterPodsStatusForRegistry(podsList *v1.SteveCollection, substring string) (bool, error) {

	for _, pod := range podsList.Data {
		podStatus := &corev1.PodStatus{}
		err := v1.ConvertToK8sType(pod.Status, podStatus)
		if err != nil {
			return false, err
		}
		podPhase := podStatus.Phase
		if podPhase != "Succeeded" && podPhase != "Running" {
			return false, nil
		}
	}
	return true, nil
}
