package workloads

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/kubectl"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rancher/shepherd/pkg/wrangler"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	revisionAnnotation = "deployment.kubernetes.io/revision"
	nginxImageName     = "nginx"
	ubuntuImageName    = "ubuntu"
	redisImageName     = "redis"
	podSteveType       = "pod"
)

func validateDeploymentUpgrade(t *testing.T, client *rancher.Client, clusterName string, namespaceName string, appv1Deployment *appv1.Deployment, expectedRevision string, image string, expectedReplicas int) {
	log.Info("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(client, clusterName, namespaceName, metav1.ListOptions{
		FieldSelector:  "metadata.name=" + appv1Deployment.Name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	})
	require.NoError(t, err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterName, namespaceName, appv1Deployment)
	require.NoError(t, err)

	log.Infof("Verifying rollout history by revision %s", expectedRevision)
	err = verifyDeploymentAgainstRolloutHistory(client, clusterName, namespaceName, appv1Deployment.Name, expectedRevision)
	require.NoError(t, err)

	log.Infof("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterName, namespaceName, image)
	require.NoError(t, err)
	require.Equal(t, expectedReplicas, countPods)
}

func validateDeploymentScale(t *testing.T, client *rancher.Client, clusterName string, namespaceName string, scaleDeployment *appv1.Deployment, image string, expectedReplicas int) {
	log.Info("Waiting deployment comes up active")
	err := charts.WatchAndWaitDeployments(client, clusterName, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + scaleDeployment.Name,
	})
	require.NoError(t, err)

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterName, namespaceName, scaleDeployment)
	require.NoError(t, err)

	log.Infof("Counting all pods running by image %s", image)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterName, namespaceName, image)
	require.NoError(t, err)
	require.Equal(t, expectedReplicas, countPods)
}

func rollbackDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, revision int) (string, error) {
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return "", err
	}

	namespaceClient := steveclient.SteveType(podSteveType).NamespacedSteveClient(namespaceName)
	podsResp, err := namespaceClient.List(nil)
	if err != nil {
		return "", err
	}

	//Collect the pod IDs that are expected to be deleted after the rollback
	expectBeDeletedIds := []string{}
	for _, podResp := range podsResp.Data {
		expectBeDeletedIds = append(expectBeDeletedIds, podResp.ID)
	}

	//Execute the roolback
	deploymentCmd := fmt.Sprintf("deployment.apps/%s", deploymentName)
	revisionCmd := fmt.Sprintf("--to-revision=%s", strconv.Itoa(revision))
	execCmd := []string{"kubectl", "rollout", "undo", "-n", namespaceName, deploymentCmd, revisionCmd}
	logCmd, err := kubectl.Command(client, nil, clusterID, execCmd, "")
	if err != nil {
		return "", err
	}

	backoff := kwait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1,
		Jitter:   0,
		Steps:    10,
	}

	//Waiting for all expectedToBeDeletedIds to be deleted
	err = kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		for _, id := range expectBeDeletedIds {
			//If the expected delete ID doesn't exist, it should be ignored
			podResp, err := namespaceClient.ByID(id)
			if err != nil && strings.Contains(err.Error(), "404 Not Found") {
				continue
			}
			if err != nil {
				return false, err
			}
			if podResp != nil {
				return false, nil
			}
		}
		return true, nil
	})

	return logCmd, err
}

func verifyDeploymentAgainstRolloutHistory(client *rancher.Client, clusterID, namespaceName string, deploymentName string, expectedRevision string) error {
	var wranglerContext *wrangler.Context
	var err error

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	wranglerContext = client.WranglerContext
	if clusterID != "local" {
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

func verifyOrchestrationStatus(client *rancher.Client, clusterID, namespaceName string, deploymentName string, isPaused bool) error {
	var wranglerContext *wrangler.Context
	var err error

	err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return err
	}

	wranglerContext = client.WranglerContext
	if clusterID != "local" {
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
