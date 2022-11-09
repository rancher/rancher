package workloads

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

// PodGroupVersion is the required Group Version for accessing pods in a cluster,
// using the dynamic client.
var PodGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "pods",
}

// StatusPods is a helper function that uses the dynamic client to list pods on a namespace for a specific cluster with its list options.
func StatusPods(client *rancher.Client, clusterID string, listOpts metav1.ListOptions) ([]string, []error) {
	var podList []corev1.Pod

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, []error{err}
	}
	podResource := dynamicClient.Resource(PodGroupVersionResource)
	pods, err := podResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, []error{err}
	}

	for _, unstructuredPod := range pods.Items {
		newPod := &corev1.Pod{}
		err := scheme.Scheme.Convert(&unstructuredPod, newPod, unstructuredPod.GroupVersionKind())
		if err != nil {
			return nil, []error{err}
		}

		podList = append(podList, *newPod)
	}

	var podResults []string
	var podErrors []error

	for _, pod := range podList {
		podStatus := pod.Status.Phase
		if podStatus == "Succeeded" || podStatus == "Running" {
			podResults = append(podResults, fmt.Sprintf("INFO: %s: %s\n", pod.Name, podStatus))
		} else {
			podErrors = append(podErrors, fmt.Errorf("ERROR: %s: %s\n", pod.Name, podStatus))
		}
	}
	return podResults, podErrors
}

// WaitPodTerminated is a helper function that uses wait.Poll() to verify if all pods with podName
// have terminated correctly or if there is still a pod runnnig.
func WaitPodTerminated(client *rancher.Client, clusterID string, podName string) bool {
	err := wait.Poll(5*time.Second, 2*time.Minute, func() (done bool, err error) {
		podResults, _ := StatusPods(client, clusterID, metav1.ListOptions{})
		for _, pod := range podResults {
			p := strings.Split(pod, ": ")
			if strings.HasPrefix(p[1], podName) && strings.HasPrefix(p[2], "Running") {
				return false, nil
			}
		}
		return true, nil
	})

	if err != nil {
		return false
	} else {
		return true
	}
}
