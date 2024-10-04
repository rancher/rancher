package workloads

import (
	"errors"
	"fmt"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/workloads/cronjob"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/rancher/tests/v2/actions/workloads/statefulset"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kubectl_output_err = "No log output was recieved from kubectl"
	nginxImage         = "nginx"
	ubuntuImage        = "ubuntu"
	redisImage         = "redis"
	cronExpression     = "*/1 * * * *"
)

type WorkloadTestFunc func(client *rancher.Client, clusterID string) error

type WorkloadTest struct {
	name     string
	testFunc WorkloadTestFunc
}

func GetAllWorkloadTests() []WorkloadTest {
	tests := []WorkloadTest{
		{"WorkloadDeploymentTest", WorkloadDeploymentTest},
		{"WorkloadSideKickTest", WorkloadSideKickTest},
		{"WorkloadDaemonSetTest", WorkloadDaemonSetTest},
		{"WorkloadCronjobTest", WorkloadCronjobTest},
		{"WorkloadStatefulsetTest", WorkloadStatefulsetTest},
		{"WorkloadUpgradeTest", WorkloadUpgradeTest},
		{"WorkloadPodScaleUpTest", WorkloadPodScaleUpTest},
		{"WorkloadPodScaleDownTest", WorkloadPodScaleDownTest},
		{"WorkloadPauseOrchestrationTest", WorkloadPauseOrchestrationTest},
	}

	return tests
}

// WorkloadDeploymentTest creates a deployment and validates that it is successfully deployed.
func WorkloadDeploymentTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deployment")
	_, err = deployment.CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false)

	return err
}

// WorkloadSideKickTest updates a deployment and validates the result.
func WorkloadSideKickTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deployment")
	createdDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false)
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, createdDeployment)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImage,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)

	createdDeployment.Spec.Template.Spec.Containers = append(createdDeployment.Spec.Template.Spec.Containers, newContainerTemplate)

	log.Info("Updating image deployment")
	updatedDeployment, err := deployment.UpdateDeployment(client, clusterID, namespace.Name, createdDeployment)
	if err != nil {
		return err
	}

	log.Info("Waiting for deployment to become active")
	err = charts.WatchAndWaitDeployments(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + updatedDeployment.Name,
	})
	if err != nil {
		return err
	}

	log.Info("Waiting for all pods to be running")
	err = pods.WatchAndWaitPodContainerRunning(client, clusterID, namespace.Name, updatedDeployment)

	return err
}

// WorkloadDaemonSetTest creates a daemonset and validates that it is successfully deployed.
func WorkloadDaemonSetTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deamonset")
	createdDaemonset, err := daemonset.CreateDaemonset(client, clusterID, namespace.Name, 1, "", "", false, false)
	if err != nil {
		return err
	}

	log.Info("Waiting for deamonset to become active")
	err = charts.WatchAndWaitDaemonSets(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + createdDaemonset.Name,
	})

	return err
}

// WorkloadCronjobTest creates a cron job and validates that it is successfully deployed.
func WorkloadCronjobTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImage,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	log.Info("Creating a new cronjob")
	cronJobTemplate, err := cronjob.CreateCronjob(client, clusterID, namespace.Name, "*/1 * * * *", podTemplate)
	if err != nil {
		return err
	}

	log.Info("Waiting for cronjob to become active")
	err = cronjob.WatchAndWaitCronjob(client, clusterID, namespace.Name, cronJobTemplate)

	return err
}

// WorkloadStatefulsetTest creates a stateful set and validates that it is successfully deployed.
func WorkloadStatefulsetTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	containerName := namegen.AppendRandomString("test-container")
	pullPolicy := corev1.PullAlways

	containerTemplate := workloads.NewContainer(
		containerName,
		nginxImage,
		pullPolicy,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	podTemplate := workloads.NewPodTemplate(
		[]corev1.Container{containerTemplate},
		[]corev1.Volume{},
		[]corev1.LocalObjectReference{},
		nil,
		nil,
	)

	log.Info("Creating a new statefulset")
	statefulsetTemplate, err := statefulset.CreateStatefulset(client, clusterID, namespace.Name, podTemplate, 1)
	if err != nil {
		return err
	}

	log.Info("Waiting statefulset to become active")
	err = charts.WatchAndWaitStatefulSets(client, clusterID, namespace.Name, metav1.ListOptions{
		FieldSelector: "metadata.name=" + statefulsetTemplate.Name,
	})

	return err
}

// WorkloadUpgradeTest upgrades a deployment and validates the result.
func WorkloadUpgradeTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deployment")
	upgradeDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 2, "", "", false, false)
	if err != nil {
		return err
	}

	validateDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "1", nginxImage, 2)

	containerName := namegen.AppendRandomString("update-test-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImage,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	upgradeDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment")
	upgradeDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, upgradeDeployment)
	if err != nil {
		return err
	}

	validateDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "2", redisImage, 2)

	containerName = namegen.AppendRandomString("update-test-container-two")
	newContainerTemplate = workloads.NewContainer(containerName,
		ubuntuImage,
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

	log.Info("Updating deployment")
	_, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, upgradeDeployment)
	if err != nil {
		return err
	}

	validateDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "3", ubuntuImage, 2)

	log.Info("Rolling back deployment")
	logRollback, err := rollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 1)
	if err != nil {
		return err
	}

	if logRollback == "" {
		return errors.New(kubectl_output_err)
	}

	validateDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "4", nginxImage, 2)

	log.Info("Rolling back deployment")
	logRollback, err = rollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 2)
	if err != nil {
		return err
	}

	if logRollback == "" {
		return errors.New(kubectl_output_err)
	}

	validateDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "5", redisImage, 2)

	log.Info("Rolling back deployment")
	logRollback, err = rollbackDeployment(client, clusterID, namespace.Name, upgradeDeployment.Name, 3)
	if err != nil {
		return err
	}

	if logRollback == "" {
		return errors.New(kubectl_output_err)
	}

	validateDeploymentUpgrade(client, clusterID, namespace.Name, upgradeDeployment, "6", ubuntuImage, 2)

	return err
}

// WorkloadPodScaleUpTest scales up pod replicas and validates the result.
func WorkloadPodScaleUpTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deployment")
	scaleUpDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 1, "", "", false, false)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImage, 1)

	replicas := int32(2)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleUpDeployment)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImage, 2)

	replicas = int32(3)
	scaleUpDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleUpDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleUpDeployment)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, scaleUpDeployment, nginxImage, 3)

	return err
}

// WorkloadPodScaleDownTest scales down pod replicas and validates the result.
func WorkloadPodScaleDownTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deployment")
	scaleDownDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 3, "", "", false, false)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImage, 3)

	replicas := int32(2)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleDownDeployment)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImage, 2)

	replicas = int32(1)
	scaleDownDeployment.Spec.Replicas = &replicas

	log.Info("Updating deployment replicas")
	scaleDownDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, scaleDownDeployment)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, scaleDownDeployment, nginxImage, 1)

	return err
}

// WorkloadPauseOrchestrationTest performs an orchestration pause and validates the result
func WorkloadPauseOrchestrationTest(client *rancher.Client, clusterID string) error {
	log.Info("Creating a new project and namespace")
	_, namespace, err := projectsapi.CreateProjectAndNamespace(client, clusterID)
	if err != nil {
		return err
	}

	log.Info("Creating a new deployment")
	pauseDeployment, err := deployment.CreateDeployment(client, clusterID, namespace.Name, 2, "", "", false, false)
	if err != nil {
		return err
	}

	validateDeploymentScale(client, clusterID, namespace.Name, pauseDeployment, nginxImage, 2)

	log.Info("Pausing orchestration")
	pauseDeployment.Spec.Paused = true
	pauseDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment)
	if err != nil {
		return err
	}

	log.Info("Verifying orchestration is paused")
	err = validateOrchestrationStatus(client, clusterID, namespace.Name, pauseDeployment.Name, true)
	if err != nil {
		return err
	}

	replicas := int32(3)
	pauseDeployment.Spec.Replicas = &replicas
	containerName := namegen.AppendRandomString("pause-redis-container")
	newContainerTemplate := workloads.NewContainer(containerName,
		redisImage,
		corev1.PullAlways,
		[]corev1.VolumeMount{},
		[]corev1.EnvFromSource{},
		nil,
		nil,
		nil,
	)
	pauseDeployment.Spec.Template.Spec.Containers = []corev1.Container{newContainerTemplate}

	log.Info("Updating deployment image and replicas")
	pauseDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment)
	if err != nil {
		return err
	}

	log.Info("Verifying that the deployment was not updated and the replica count was increased")
	log.Infof("Counting all pods running by image %s", nginxImage)
	countPods, err := pods.CountPodContainerRunningByImage(client, clusterID, namespace.Name, nginxImage)
	if err != nil {
		return err
	}

	if int(replicas) != countPods {
		err_msg := fmt.Sprintf("pod count: %d does not equal expected number of replicas: %d", countPods, int(replicas))
		return errors.New(err_msg)
	}

	log.Info("Activing orchestration")
	pauseDeployment.Spec.Paused = false
	pauseDeployment, err = deployment.UpdateDeployment(client, clusterID, namespace.Name, pauseDeployment)

	validateDeploymentScale(client, clusterID, namespace.Name, pauseDeployment, redisImage, int(replicas))

	log.Info("Verifying orchestration is active")
	err = validateOrchestrationStatus(client, clusterID, namespace.Name, pauseDeployment.Name, false)
	if err != nil {
		return err
	}

	log.Infof("Counting all pods running by image %s", redisImage)
	countPods, err = pods.CountPodContainerRunningByImage(client, clusterID, namespace.Name, redisImage)
	if err != nil {
		return err
	}

	if int(replicas) != countPods {
		err_msg := fmt.Sprintf("pod count: %d does not equal expected number of replicas: %d", countPods, int(replicas))
		return errors.New(err_msg)
	}

	return err
}
