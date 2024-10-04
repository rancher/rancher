package workloads

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/pkg/wrangler"
	log "github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	revisionAnnotation = "deployment.kubernetes.io/revision"
	localClusterID     = "local"
)

// rollbackDeployment reverts the deployment to a previous revision
func rollbackDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, revision int) (string, error) {
	deploymentCmd := fmt.Sprintf("deployment.apps/%s", deploymentName)
	revisionCmd := fmt.Sprintf("--to-revision=%s", strconv.Itoa(revision))
	execCmd := []string{"kubectl", "rollout", "undo", "-n", namespaceName, deploymentCmd, revisionCmd}
	logCmd, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	return logCmd, err
}

// validateDeploymentUpgrade validates that a deployment updated successfully.
func validateDeploymentUpgrade(client *rancher.Client, clusterID string, namespaceName string, appv1Deployment *appv1.Deployment, expectedRevision string, image string, expectedContainerCount int) error {
	log.Info("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + appv1Deployment.Name,
	})
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespaceName, appv1Deployment)
	if err != nil {
		return err
	}

	log.Infof("Verifying rollout history by revision %s", expectedRevision)
	err = validateDeploymentAgainstRolloutHistory(client, clusterID, namespaceName, appv1Deployment.Name, expectedRevision)
	if err != nil {
		return err
	}

	log.Infof("Counting all running pods by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterID, namespaceName, image)
	if err != nil {
		return err
	}

	if expectedContainerCount != countPods {
		err_msg := fmt.Sprintf("pod count: %d does not equal expected pod count: %d", countPods, expectedContainerCount)
		return errors.New(err_msg)
	}

	return err
}

// validateDeploymentScale validates that a deployment sclaed to the appropriate node count
func validateDeploymentScale(client *rancher.Client, clusterID string, namespaceName string, scaleDeployment *appv1.Deployment, image string, expectedReplicas int) error {
	log.Info("Waiting for deployment to become active")
	err := charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + scaleDeployment.Name,
	})
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespaceName, scaleDeployment)
	if err != nil {
		return err
	}

	log.Infof("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterID, namespaceName, image)
	if err != nil {
		return err
	}

	if expectedReplicas != countPods {
		err_msg := fmt.Sprintf("pod count: %d does not equal expected number of replicas: %d", countPods, expectedReplicas)
		return errors.New(err_msg)
	}

	return err
}

// validateDeploymentAgainstRolloutHistory validates that a deployment has a specific revision
func validateDeploymentAgainstRolloutHistory(client *rancher.Client, clusterID, namespaceName string, deploymentName string, expectedRevision string) error {
	var wranglerContext *wrangler.Context
	var err error

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	wranglerContext = client.WranglerContext
	if clusterID != localClusterID {
		wranglerContext, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return err
		}
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if latestDeployment.ObjectMeta.Annotations == nil {
		return errors.New("revision empty")
	}

	revision := latestDeployment.ObjectMeta.Annotations[revisionAnnotation]

	if revision != expectedRevision {
		return errors.New("revision not found")
	}

	return nil
}

// validateOrchestrationStatus validates that a deployment is in the paused state
func validateOrchestrationStatus(client *rancher.Client, clusterID, namespaceName string, deploymentName string, isPaused bool) error {
	var wranglerContext *wrangler.Context
	var err error

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	wranglerContext = client.WranglerContext
	if clusterID != localClusterID {
		wranglerContext, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return err
		}
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if isPaused && !latestDeployment.Spec.Paused {
		return errors.New("the orchestration is active")
	}

	if !isPaused && latestDeployment.Spec.Paused {
		return errors.New("the orchestration is paused")
	}

	return nil
}
