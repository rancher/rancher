package pods

import (
	"fmt"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	PodResourceSteveType = "pod"
)

// StatusPods is a helper function that uses the steve client to list pods on a namespace for a specific cluster
// and return the statuses in a list of strings
func StatusPods(client *rancher.Client, clusterID string) ([]string, []error) {
	downstreamClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, []error{err}
	}

	var podResults []string
	var podErrors []error

	steveClient := downstreamClient.SteveType(PodResourceSteveType)
	err = kwait.Poll(5*time.Second, 15*time.Minute, func() (done bool, err error) {
		podResults = []string{}
		podErrors = []error{}

		pods, err := steveClient.List(nil)
		if err != nil {
			podErrors = append(podErrors, err)
			return false, nil
		}

		podResults = append(podResults, "Pod's Status: \n")

		for _, pod := range pods.Data {
			podResult, podError, err := CheckPodStatus(&pod)
			if err != nil {
				// not returning the error in this case, as it could cause a false positive if we start polling too early.
				return false, nil
			}
			if podError != nil {
				podErrors = append(podErrors, podError)
				return false, nil
			}
			if podResult != "" {
				podResults = append(podResults, podResult)
			}
		}

		if len(podResults) > 0 && len(podErrors) == 0 {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return nil, []error{err}
	}

	return podResults, podErrors
}

// CheckPodStatus is a helper function that uses the steve client to check the status of a single pod
func CheckPodStatus(pod *v1.SteveAPIObject) (podResults string, podError error, err error) {
	podStatus := &corev1.PodStatus{}
	err = v1.ConvertToK8sType(pod.Status, podStatus)
	if err != nil {
		return "", nil, err
	}
	if podStatus.ContainerStatuses == nil || len(podStatus.ContainerStatuses) == 0 {
		return fmt.Sprintf("WARN: %s: Container Status is Empty \n", pod.Name), nil, nil
	}

	image := podStatus.ContainerStatuses[0].Image
	phase := podStatus.Phase

	if phase == corev1.PodFailed || phase == corev1.PodUnknown {
		logrus.Infof("Pod %s: Not active | Image %s", pod.Name, image)
		// Do not report as error if pod is in failed state due to container termination
		for _, containerStatus := range podStatus.ContainerStatuses {
			if containerStatus.State.Terminated == nil {
				return "", fmt.Errorf("ERROR: %s: %s", pod.Name, podStatus), nil
			}
		}
		return fmt.Sprintf("INFO: TERMINATED %s: %s\n", pod.Name, podStatus), nil, nil
	}

	if phase == corev1.PodRunning {
		logrus.Infof("Pod %s: Active | Image: %s", pod.Name, image)
		return fmt.Sprintf("INFO: %s: %s\n", pod.Name, podStatus), nil, nil
	}

	return "", nil, nil
}
