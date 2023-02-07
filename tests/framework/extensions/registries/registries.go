package registries

import (
	"strings"
	"time"

	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

func CheckAllClusterPodsForRegistryPrefix(podsList *v1.SteveCollection, substring string) (bool, error) {

	for _, pod := range podsList.Data {
		podSpec := &corev1.PodSpec{}
		err := v1.ConvertToK8sType(pod.Spec, podSpec)
		if err != nil {
			return false, err
		}
		for _, container := range podSpec.Containers {
			log.Infoln(container.Image)
			if !strings.Contains(container.Image, substring) {
				return false, nil
			}
		}
	}
	return true, nil
}

func CheckAllClusterPodsStatusForRegistry(podsList *v1.SteveCollection) (bool, error) {

	err := kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		for _, pod := range podsList.Data {
			podStatus := &corev1.PodStatus{}
			err := v1.ConvertToK8sType(pod.Status, podStatus)
			if err != nil {
				return false, err
			}
			podPhase := podStatus.Phase
			if podPhase != "Succeeded" && podPhase != "Running" {
				log.Infoln(podPhase)
				log.Infof("Reason: %s", podStatus.Reason)
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}
