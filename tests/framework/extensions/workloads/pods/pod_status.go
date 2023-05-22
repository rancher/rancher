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
	err = kwait.Poll(1*time.Second, 10*time.Minute, func() (done bool, err error) {
		podResults = []string{}
		podErrors = []error{}

		pods, err := steveClient.List(nil)
		if err != nil {
			podErrors = append(podErrors, err)
			return false, nil
		}

		podResults = append(podResults, "Pod's Status: \n")

		for _, pod := range pods.Data {
			podStatus := &corev1.PodStatus{}
			err = v1.ConvertToK8sType(pod.Status, podStatus)
			if err != nil {
				return false, err
			}

			phase := podStatus.Phase
			if phase == corev1.PodFailed || phase == corev1.PodUnknown {
				podErrors = append(podErrors, fmt.Errorf("ERROR: %s: %s", pod.Name, podStatus))
				logrus.Infof("Pod %s is not in an active state", pod.Name)
				return false, nil
			} else if phase == corev1.PodRunning {
				podResults = append(podResults, fmt.Sprintf("INFO: %s: %s\n", pod.Name, podStatus))
				logrus.Infof("Pod %s is in an active state!", pod.Name)
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		return nil, []error{err}
	}

	return podResults, podErrors
}
