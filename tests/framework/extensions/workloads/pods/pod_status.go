package pods

import (
	"fmt"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	PodResourceSteveType = "pod"
)

// StatusPods is a helper function that uses the steve client to list pods on a namespace for a specific cluster
// and return the statuses in a list of strings
func StatusPods(client *rancher.Client, clusterID string) []error {
	downstreamClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return []error{err}
	}

	var podErrors []error

	steveClient := downstreamClient.SteveType(PodResourceSteveType)
	err = kwait.Poll(5*time.Second, 15*time.Minute, func() (done bool, err error) {
		// emptying pod errors every time we poll so that we don't return stale errors
		podErrors = []error{}

		pods, err := steveClient.List(nil)
		if err != nil {
			// not returning the error in this case, as it could cause a false positive if we start polling too early.
			return false, nil
		}

		for _, pod := range pods.Data {
			isReady, err := IsPodReady(&pod)
			if !isReady {
				// not returning the error in this case, as it could cause a false positive if we start polling too early.
				return false, nil
			}

			if err != nil {
				podErrors = append(podErrors, err)
			}
		}
		return true, nil
	})

	if err != nil {
		podErrors = append(podErrors, err)
	}

	return podErrors
}

func IsPodReady(pod *v1.SteveAPIObject) (bool, error) {
	podStatus := &corev1.PodStatus{}
	err := v1.ConvertToK8sType(pod.Status, podStatus)
	if err != nil {
		return false, err
	}

	if podStatus.ContainerStatuses == nil || len(podStatus.ContainerStatuses) == 0 {
		return false, nil
	}

	phase := podStatus.Phase

	if phase == corev1.PodPending {
		return false, nil
	}

	if phase == corev1.PodFailed || phase == corev1.PodUnknown {
		var errorMessage string
		for _, containerStatus := range podStatus.ContainerStatuses {
			// Rancher deploys multiple hlem-operation jobs to do the same task. If one job succeeds, the others end in a terminated status.
			if containerStatus.State.Terminated == nil {
				errorMessage += fmt.Sprintf("ERROR: %s: %s\n", pod.Name, podStatus)
			}
		}

		if errorMessage != "" {
			return true, fmt.Errorf(errorMessage)
		}
	}

	// Pod is running or has succeeded
	return true, nil
}
