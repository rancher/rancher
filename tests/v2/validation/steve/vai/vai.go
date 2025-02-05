package vai

import (
	"fmt"
	"github.com/rancher/shepherd/clients/rancher"
	"strings"
)

const (
	randomStringLength = 8
	uiSQLCacheResource = "ui-sql-cache"
)

type SupportedWithVai interface {
	SupportedWithVai() bool
}

func isVaiEnabled(client *rancher.Client) (bool, error) {
	managementClient := client.Steve.SteveType("management.cattle.io.feature")
	steveCacheFlagResp, err := managementClient.ByID(uiSQLCacheResource)
	if err != nil {
		return false, err
	}
	spec, ok := steveCacheFlagResp.Spec.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("unable to access Spec field")
	}

	valueInterface, exists := spec["value"]
	if !exists {
		return false, nil
	}

	if valueInterface == nil {
		return false, nil
	}

	value, ok := valueInterface.(bool)
	if !ok {
		return false, fmt.Errorf("value field is not a boolean")
	}

	return value, nil
}

func filterTestCases[T SupportedWithVai](testCases []T, vaiEnabled bool) []T {
	if !vaiEnabled {
		return testCases
	}

	var supported []T
	for _, tc := range testCases {
		if tc.SupportedWithVai() {
			supported = append(supported, tc)
		}
	}
	return supported
}

func listRancherPods(client *rancher.Client) ([]string, error) {
	// Use the Steve client to list all pods
	podList, err := client.Steve.SteveType("pod").NamespacedSteveClient("cattle-system").List(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var rancherPodNames []string
	for _, pod := range podList.Data {

		// Filter for Rancher pods, excluding the webhook
		if pod.Labels["app"] == "rancher" && !strings.Contains(pod.Name, "webhook") {
			rancherPodNames = append(rancherPodNames, pod.Name)
		}
	}
	return rancherPodNames, nil
}
