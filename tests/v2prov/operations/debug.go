package operations

import (
	"bufio"
	"context"
	"fmt"
	"time"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// downstreamDebugTimeout bounds each downstream API call, as the downstream apiserver may be unreachable after a failed operation.
const downstreamDebugTimeout = 30 * time.Second

// GatherDownstreamDebugData gathers debug data from the downstream cluster itself: every pod in the cluster and the logs of the
// cattle-cluster-agent pods. It returns a gzip compressed + base64 encoded string of the json, in the same format as
// cluster.GatherDebugData.
func GatherDownstreamDebugData(clients *clients.Clients, c *v1.Cluster) (string, error) {
	clientset, err := getDownstreamClientset(clients, c)
	if err != nil {
		return "", fmt.Errorf("failed to get downstream clientset for cluster %s/%s: %w", c.Namespace, c.Name, err)
	}

	pods, newErr := listPods(clientset, metav1.NamespaceAll, "")
	if newErr != nil {
		logrus.Errorf("failed to list downstream pods for cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
		pods = nil
	}

	var podLogs = make(map[string]map[string]string)
	agentPods, newErr := listPods(clientset, "cattle-system", "app=cattle-cluster-agent")
	if newErr != nil {
		logrus.Errorf("failed to list downstream cattle-cluster-agent pods for cluster %s/%s to print error: %v", c.Namespace, c.Name, newErr)
	} else {
		for _, pod := range agentPods.Items {
			podLogs[pod.Name] = populateDownstreamPodLogs(clientset, pod)
		}
	}

	return snapshotutil.CompressInterface(map[string]interface{}{
		"pods":    pods,
		"podLogs": podLogs,
	})
}

// PrintDebugData prints the local and downstream debug data bundles for the cluster. operation describes what failed and is
// included in the printed header.
func PrintDebugData(clients *clients.Clients, c *v1.Cluster, operation string) {
	data, newErr := cluster.GatherDebugData(clients, c)
	if newErr != nil {
		logrus.Error(newErr)
	}
	fmt.Printf("cluster %s %s failed\ncluster %s test data bundle: \n%s\n", c.Name, operation, c.Name, data)

	downstreamData, newErr := GatherDownstreamDebugData(clients, c)
	if newErr != nil {
		logrus.Error(newErr)
	}
	fmt.Printf("cluster %s downstream test data bundle: \n%s\n", c.Name, downstreamData)
}

func listPods(clientset kubernetes.Interface, namespace, selector string) (*corev1.PodList, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), downstreamDebugTimeout)
	defer cancel()
	return clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
}

// populateDownstreamPodLogs collects the current logs of the pod, and the logs of the previous container instance if the pod has
// restarted, since an agent that crashloops after a restore usually logs the cause before exiting.
func populateDownstreamPodLogs(clientset kubernetes.Interface, pod corev1.Pod) map[string]string {
	var logMap = make(map[string]string)

	logs, err := getDownstreamPodLogs(clientset, pod.Namespace, pod.Name, false)
	if err != nil {
		logrus.Errorf("error while retrieving downstream pod logs: %v", err)
	} else {
		logMap["logs"] = logs
	}

	for _, status := range pod.Status.ContainerStatuses {
		if status.RestartCount == 0 {
			continue
		}
		previousLogs, err := getDownstreamPodLogs(clientset, pod.Namespace, pod.Name, true)
		if err != nil {
			logrus.Errorf("error while retrieving downstream previous pod logs: %v", err)
		} else {
			logMap["previousLogs"] = previousLogs
		}
		break
	}

	return logMap
}

// getDownstreamPodLogs gathers the logs from the specified downstream pod in a manner similar to `kubectl logs`, encoded the same
// way as cluster.GatherDebugData pod logs.
func getDownstreamPodLogs(clientset kubernetes.Interface, podNamespace, podName string, previous bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), downstreamDebugTimeout)
	defer cancel()
	stream, err := clientset.CoreV1().Pods(podNamespace).GetLogs(podName, &corev1.PodLogOptions{Previous: previous}).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("error streaming downstream pod logs for pod %s/%s: %v", podNamespace, podName, err)
	}
	defer stream.Close()

	reader := bufio.NewScanner(stream)
	var logs string
	for reader.Scan() {
		logs = logs + fmt.Sprintf("%sSnewlineG", reader.Text())
	}
	return snapshotutil.CompressInterface(logs)
}
