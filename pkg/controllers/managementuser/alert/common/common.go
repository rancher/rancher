package common

import (
	"fmt"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/sirupsen/logrus"
)

func GetRuleID(groupID string, ruleName string) string {
	return fmt.Sprintf("%s_%s", groupID, ruleName)
}

func GetGroupID(namespace, name string) string {
	return fmt.Sprintf("%s:%s", namespace, name)
}

func GetAlertManagerSecretName(appName string) string {
	return fmt.Sprintf("alertmanager-%s", appName)
}

func GetAlertManagerDaemonsetName(appName string) string {
	return fmt.Sprintf("alertmanager-%s", appName)
}

func formatProjectDisplayName(projectDisplayName, projectID string) string {
	return fmt.Sprintf("%s (ID: %s)", projectDisplayName, projectID)
}

func formatClusterDisplayName(clusterDisplayName, clusterID string) string {
	return fmt.Sprintf("%s (ID: %s)", clusterDisplayName, clusterID)
}

func GetClusterDisplayName(clusterName string, clusterLister v3.ClusterLister) string {
	cluster, err := clusterLister.Get("", clusterName)
	if err != nil {
		logrus.Warnf("Failed to get cluster for %s: %v", clusterName, err)
		return clusterName
	}

	return formatClusterDisplayName(cluster.Spec.DisplayName, clusterName)
}

func GetProjectDisplayName(projectID string, projectLister v3.ProjectLister) string {
	clusterName, projectName := ref.Parse(projectID)
	project, err := projectLister.Get(clusterName, projectName)
	if err != nil {
		logrus.Warnf("Failed to get project %s: %v", projectID, err)
		return projectID
	}

	return formatProjectDisplayName(project.Spec.DisplayName, projectID)
}
