package deployment

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	active              = "active"
	defaultNamespace    = "default"
	port                = "port"
	DeploymentSteveType = "apps.deployment"
	imageName           = "nginx"
	historyHeaderLength = 2
	revisionNumberIndex = 0
	historyHeader       = "REVISION  CHANGE-CAUSE"
	revisionsIndex      = 1
)

// CreateDeployment is a helper to create a deployment with or without a secret/configmap
func CreateDeployment(client *rancher.Client, clusterID, namespaceName string, replicaCount int, secretName, configMapName string, useEnvVars, useVolumes bool) (*appv1.Deployment, error) {
	deploymentName := namegen.AppendRandomString("testdeployment")
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways
	replicas := int32(replicaCount)

	var podTemplate corev1.PodTemplateSpec

	if secretName != "" || configMapName != "" {
		podTemplate = pods.NewPodTemplateWithConfig(secretName, configMapName, useEnvVars, useVolumes)
	} else {
		containerTemplate := workloads.NewContainer(
			containerName,
			imageName,
			pullPolicy,
			[]corev1.VolumeMount{},
			[]corev1.EnvFromSource{},
			nil,
			nil,
			nil,
		)
		podTemplate = workloads.NewPodTemplate(
			[]corev1.Container{containerTemplate},
			[]corev1.Volume{},
			[]corev1.LocalObjectReference{},
			nil,
			nil,
		)
	}

	createdDeployment, err := deployments.CreateDeployment(client, clusterID, deploymentName, namespaceName, podTemplate, replicas)
	if err != nil {
		return nil, err
	}

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDeployment.Name,
	})

	return createdDeployment, err
}

// UpdateDeployment is a helper to update deployments
func UpdateDeployment(client *rancher.Client, clusterID, namespaceName string, deployment *appv1.Deployment) (*appv1.Deployment, error) {
	wranglerContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return nil, err
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deployment.ResourceVersion = latestDeployment.ResourceVersion

	updatedDeployment, err := wranglerContext.Apps.Deployment().Update(deployment)
	if err != nil {
		return nil, err
	}

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})

	return updatedDeployment, err
}

// RolbackDeployment is a helper to rollback deployments
func RollbackDeployment(client *rancher.Client, clusterID, namespaceName string, deployment *appv1.Deployment, revision int) (string, error) {
	deploymentCmd := fmt.Sprintf("deployment.apps/%s", deployment.Name)
	revisionCmd := fmt.Sprintf("--to-revision=%s", strconv.Itoa(revision))
	execCmd := []string{"kubectl", "rollout", "undo", "-n", namespaceName, deploymentCmd, revisionCmd}
	logCmd, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	return logCmd, err
}

// GetRolloutHistoryDeployment is a helper to get rollout history deployment
func GetRolloutHistoryDeployment(client *rancher.Client, clusterID, namespaceName string, deployment *appv1.Deployment) ([]string, error) {
	deploymentCmd := fmt.Sprintf("deployment.apps/%s", deployment.Name)
	execCmd := []string{"kubectl", "rollout", "history", "-n", namespaceName, deploymentCmd}
	logCmd, err := kubectl.Command(client, nil, clusterID, execCmd, "")

	if err != nil {
		return []string{}, err
	}

	historyHeaderSplit := strings.Split(logCmd, historyHeader)

	if len(historyHeaderSplit) < historyHeaderLength {
		return []string{}, err
	}

	histories := strings.Split(historyHeaderSplit[revisionsIndex], "\n")
	revisions := []string{}

	for _, history := range histories {
		revision := strings.SplitAfter(history, "")
		if len(revision) > 0 {
			revisions = append(revisions, revision[revisionNumberIndex])
		}
	}

	return revisions, err
}
