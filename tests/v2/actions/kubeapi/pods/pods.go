package pods

import (
	"fmt"
	"strings"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	log "github.com/sirupsen/logrus"
)

const (
	maxRetries   = 5
	retryDelay   = 5 * time.Second
	scaleDelay   = 30 * time.Second
	waitTimeout  = 2 * time.Minute
	pollInterval = 5 * time.Second
)

// WaitForReadyPods waits for the specified number of pods to be ready for a given workload.
func WaitForReadyPods(client *rancher.Client, clusterID, namespace, workloadName string, expectedPodCount int) error {
	steveClient, err := getDownstreamClient(client, clusterID)
	if err != nil {
		return err
	}

	time.Sleep(scaleDelay)

	timeout := time.After(waitTimeout)
	tick := time.Tick(pollInterval)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for %d pods to be ready for workload %s", expectedPodCount, workloadName)
		case <-tick:
			readyCount, err := getPodReadyCount(steveClient, namespace, workloadName)
			if err != nil {
				log.Warnf("Error checking pod status: %v", err)
				continue
			}

			if readyCount == expectedPodCount {
				log.Infof("All %d pods are ready for workload %s", expectedPodCount, workloadName)
				return nil
			}

			log.Infof("Waiting for pods to be ready: %d/%d", readyCount, expectedPodCount)
		}
	}
}

// getDownstreamClient attempts to get a downstream client with retries
func getDownstreamClient(client *rancher.Client, clusterID string) (*v1.Client, error) {
	var steveClient *v1.Client
	var err error

	for retries := 0; retries < maxRetries; retries++ {
		steveClient, err = client.Steve.ProxyDownstream(clusterID)
		if err == nil {
			return steveClient, nil
		}
		log.Warnf("Failed to get downstream client (attempt %d/%d): %v", retries+1, maxRetries, err)
		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("failed to get downstream client after %d attempts: %v", maxRetries, err)
}

// getPodReadyCount returns the count of ready pods for a given workload
func getPodReadyCount(client *v1.Client, namespace, workloadName string) (int, error) {
	podList, err := client.SteveType("pod").List(nil)
	if err != nil {
		return 0, fmt.Errorf("error listing pods: %v", err)
	}

	relevantPods := filterPodsByWorkload(podList.Data, namespace, workloadName)
	log.Debugf("Found %d pods for workload %s", len(relevantPods), workloadName)

	readyCount := 0
	for _, pod := range relevantPods {
		if isPodReady(pod) {
			readyCount++
		}
	}

	return readyCount, nil
}

// filterPodsByWorkload returns pods that belong to the specified workload
func filterPodsByWorkload(pods []v1.SteveAPIObject, namespace, workloadName string) []v1.SteveAPIObject {
	var filtered []v1.SteveAPIObject
	for _, pod := range pods {
		if pod.ObjectMeta.Namespace == namespace && strings.HasPrefix(pod.ObjectMeta.Name, workloadName) {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}

// isPodReady checks if a pod is in the Ready state
func isPodReady(pod v1.SteveAPIObject) bool {
	status, ok := pod.Status.(map[string]interface{})
	if !ok {
		return false
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return false
	}

	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		if condition["type"] == "Ready" && condition["status"] == "True" {
			return true
		}
	}

	return false
}
