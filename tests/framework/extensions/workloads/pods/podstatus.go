package pods

import (
	"fmt"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	corev1 "k8s.io/api/core/v1"
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

	steveClient := downstreamClient.SteveType(PodResourceSteveType)

	pods, err := steveClient.List(&types.ListOpts{})
	if err != nil {
		return nil, []error{err}
	}

	var podResults []string
	var podErrors []error
	podResults = append(podResults, "pods Status:\n")

	for _, pod := range pods.Data {
		podStatus := &corev1.PodStatus{}
		err = v1.ConvertToK8sType(pod.Status, podStatus)
		if err != nil {
			return nil, []error{err}
		}

		phase := podStatus.Phase
		if phase == corev1.PodFailed || phase == corev1.PodUnknown {
			podErrors = append(podErrors, fmt.Errorf("ERROR: %s: %s", pod.Name, podStatus))
		} else {
			podResults = append(podResults, fmt.Sprintf("INFO: %s: %s\n", pod.Name, podStatus))
		}
	}
	return podResults, podErrors
}
