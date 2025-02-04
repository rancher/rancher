package deployment

import (
	"context"
	"errors"
	"fmt"
	"time"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/wrangler"
	"github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	revisionAnnotation = "deployment.kubernetes.io/revision"
)

// VerifyDeployment waits for a deployment to be ready in the downstream cluster
func VerifyDeployment(steveClient *steveV1.Client, deployment *steveV1.SteveAPIObject) error {
	err := kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
		deploymentResp, err := steveClient.SteveType(DeploymentSteveType).ByID(deployment.Namespace + "/" + deployment.Name)
		if err != nil {
			return false, nil
		}

		deployment := &appv1.Deployment{}
		err = steveV1.ConvertToK8sType(deploymentResp.JSONResp, deployment)
		if err != nil {

			return false, nil
		}

		if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
			return true, nil
		}

		return false, nil
	})

	return err
}

func VerifyDeploymentUpgrade(client *rancher.Client, clusterName string, namespaceName string, appv1Deployment *appv1.Deployment, expectedRevision string, image string, expectedReplicas int) error {
	logrus.Infof("Waiting for deployment %s to become active", appv1Deployment.Name)
	err := charts.WatchAndWaitDeployments(client, clusterName, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + appv1Deployment.Name,
	})
	if err != nil {
		return err
	}

	logrus.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterName, namespaceName, appv1Deployment)
	if err != nil {
		return err
	}

	logrus.Infof("Verifying rollout history by revision %s", expectedRevision)
	err = VerifyDeploymentRolloutHistory(client, clusterName, namespaceName, appv1Deployment.Name, expectedRevision)
	if err != nil {
		return err
	}

	countPods, err := pods.CountPodContainerRunningByImage(client, clusterName, namespaceName, image)
	if err != nil {
		return err
	}

	if expectedReplicas != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", expectedReplicas, countPods)
		return errors.New(err_msg)
	}

	return err
}

func VerifyDeploymentScale(client *rancher.Client, clusterName string, namespaceName string, scaleDeployment *appv1.Deployment, image string, expectedReplicas int) error {
	logrus.Infof("Waiting for deployment %s to become active", scaleDeployment.Name)
	err := charts.WatchAndWaitDeployments(client, clusterName, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + scaleDeployment.Name,
	})
	if err != nil {
		return err
	}

	logrus.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterName, namespaceName, scaleDeployment)
	if err != nil {
		return err
	}

	countPods, err := pods.CountPodContainerRunningByImage(client, clusterName, namespaceName, image)
	if err != nil {
		return err
	}

	if expectedReplicas != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", expectedReplicas, countPods)
		return errors.New(err_msg)
	}

	return err
}

func VerifyDeploymentRolloutHistory(client *rancher.Client, clusterID, namespaceName string, deploymentName string, expectedRevision string) error {
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

func VerifyOrchestrationStatus(client *rancher.Client, clusterID, namespaceName string, deploymentName string, isPaused bool) error {
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

func VerifyCreateDeployment(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	createdDeployment, err := CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false, false, true)
	if err != nil {
		return err
	}

	logrus.Infof("Creating new deployment %s", createdDeployment.Name)
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, createdDeployment)
	if err != nil {
		return err
	}

	return err
}

func VerifyCreateDeploymentSideKick(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	createdDeployment, err := CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false, false, true)
	if err != nil {
		return err
	}

	logrus.Infof("Creating new deployment %s", createdDeployment.Name)
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, createdDeployment)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	createdDeployment.Spec.Template.Spec.Containers = append(createdDeployment.Spec.Template.Spec.Containers, newContainerTemplate)

	updatedDeployment, err := UpdateDeployment(client, clusterID, namespace.Name, createdDeployment, true)
	if err != nil {
		return err
	}

	logrus.Infof("Updating deployment image, %s", createdDeployment.Name)
	err = charts.WatchAndWaitDeployments(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})
	if err != nil {
		return err
	}

	logrus.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, updatedDeployment)

	return err
}

func VerifyDeploymentUpgradeRollback(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	upgradeDeployment, err := CreateDeployment(client, clusterID, namespace.Name, 2, "", "", false, false, false, true)
	if err != nil {
		return err
	}

	logrus.Infof("Creating new deployment %s", upgradeDeployment.Name)
	err = VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "1", nginxImageName, 2)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	logrus.Infof("Updating deployment %s", upgradeDeployment.Name)
	upgradeDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, upgradeDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "2", redisImageName, 2)
	if err != nil {
		return err
	}

	containerName = namegen.AppendRandomString("update-test-container-two")
	newContainerTemplate = workloads.NewContainer(containerName,
		ubuntuImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	newContainerTemplate.TTY = true
	newContainerTemplate.Stdin = true
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	logrus.Infof("Updating deployment %s", upgradeDeployment.Name)
	_, err = UpdateDeployment(client, clusterID, namespace.Name, upgradeDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "3", ubuntuImageName, 2)
	if err != nil {
		return err
	}

	logrus.Infof("Rollback deployment %s", upgradeDeployment.Name)
	logRollback, err := RollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 1)
	if err != nil {
		return err
	}
	if logRollback == "" {
		return err
	}

	err = VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "4", nginxImageName, 2)
	if err != nil {
		return err
	}

	logrus.Infof("Rollback deployment %s", upgradeDeployment.Name)
	logRollback, err = RollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 2)
	if err != nil {
		return err
	}
	if logRollback == "" {
		return err
	}

	err = VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "5", redisImageName, 2)
	if err != nil {
		return err
	}

	logrus.Infof("Rollback deployment %s", upgradeDeployment.Name)
	logRollback, err = RollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 3)
	if err != nil {
		return err
	}
	if logRollback == "" {
		return err
	}

	err = VerifyDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "6", ubuntuImageName, 2)
	if err != nil {
		return err
	}

	return err
}

func VerifyDeploymentPodScaleUp(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	scaleUpDeployment, err := CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false, false, true)
	if err != nil {
		return err
	}

	logrus.Infof("Creating new deployment %s", scaleUpDeployment.Name)
	err = VerifyDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImageName, 1)
	if err != nil {
		return err
	}

	replicas := int32(2)
	scaleUpDeployment.Spec.Replicas = &replicas

	logrus.Info("Updating deployment replicas")
	scaleUpDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, scaleUpDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImageName, 2)
	if err != nil {
		return err
	}

	replicas = int32(3)
	scaleUpDeployment.Spec.Replicas = &replicas

	logrus.Info("Updating deployment replicas")
	scaleUpDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, scaleUpDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImageName, 3)
	if err != nil {
		return err
	}

	return err
}

func VerifyDeploymentPodScaleDown(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	scaleDownDeployment, err := CreateDeployment(client, clusterID, namespace.Name, 3, "", "", false, false, false, true)
	if err != nil {
		return err
	}

	logrus.Infof("Creating new deployment %s", scaleDownDeployment.Name)
	err = VerifyDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImageName, 3)
	if err != nil {
		return err
	}

	replicas := int32(2)
	scaleDownDeployment.Spec.Replicas = &replicas

	logrus.Info("Updating deployment replicas")
	scaleDownDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, scaleDownDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImageName, 2)
	if err != nil {
		return err
	}

	replicas = int32(1)
	scaleDownDeployment.Spec.Replicas = &replicas

	logrus.Info("Updating deployment replicas")
	scaleDownDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, scaleDownDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImageName, 1)
	if err != nil {
		return err
	}

	return err
}

func VerifyDeploymentPauseOrchestration(client *rancher.Client, clusterID string) error {
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	pauseDeployment, err := CreateDeployment(client, clusterID, namespace.Name, 2, "", "", false, false, false, true)
	if err != nil {
		return err
	}
	logrus.Infof("Creating new deployment %s", pauseDeployment.Name)
	err = VerifyDeploymentScale(client, clusterID, namespace.Name, pauseDeployment, nginxImageName, 2)
	if err != nil {
		return err
	}

	logrus.Info("Pausing orchestration")
	pauseDeployment.Spec.Paused = true
	pauseDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment, true)
	if err != nil {
		return err
	}

	err = VerifyOrchestrationStatus(client, clusterID, namespace.Name, pauseDeployment.Name, true)
	if err != nil {
		return err
	}

	replicas := int32(3)
	pauseDeployment.Spec.Replicas = &replicas
	containerName := namegen.AppendRandomString("pause-redis-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImageName,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	pauseDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	logrus.Info("Updating deployment image and replica")
	pauseDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment, true)
	if err != nil {
		return err
	}

	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, pauseDeployment)
	if err != nil {
		return err
	}

	logrus.Info("Verifying that the deployment was not updated and the replica count was increased")
	logrus.Infof("Counting all pods running by image %s", nginxImageName)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterID, namespace.Name, nginxImageName)
	if err != nil {
		return err
	}

	if int(replicas) != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", int(replicas), countPods)
		return errors.New(err_msg)
	}

	logrus.Info("Activing orchestration")
	pauseDeployment.Spec.Paused = false
	pauseDeployment, err = UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment, true)

	err = VerifyDeploymentScale(client, clusterID, namespace.Name, pauseDeployment, redisImageName, int(replicas))
	if err != nil {
		return err
	}

	err = VerifyOrchestrationStatus(client, clusterID, namespace.Name, pauseDeployment.Name, false)
	if err != nil {
		return err
	}

	logrus.Infof("Counting all pods running by image %s", redisImageName)
	countPods, err = pods.CountPodContainerRunningByImage(client, clusterID, namespace.Name, redisImageName)
	if err != nil {
		return err
	}

	if int(replicas) != countPods {
		err_msg := fmt.Sprintf("expected replica count: %d does not equal pod count: %d", int(replicas), countPods)
		return errors.New(err_msg)
	}

	return err
}
