package pods

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/kubeconfig"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	timeFormat   = "2006/01/02 15:04:05"
	imageName    = "nginx"
	podSteveType = "pod"
)

// NewPodTemplateWithConfig is a helper to create a Pod template with a secret/configmap as an environment variable or volume mount or both
func NewPodTemplateWithConfig(secretName, configMapName string, useEnvVars, useVolumes bool) corev1.PodTemplateSpec {
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways

	var envFrom []corev1.EnvFromSource
	if useEnvVars {
		if secretName != "" {
			envFrom = append(envFrom, corev1.EnvFromSource{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				},
			})
		}
		if configMapName != "" {
			envFrom = append(envFrom, corev1.EnvFromSource{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
				},
			})
		}
	}

	var volumes []corev1.Volume
	if useVolumes {
		volumeName := namegen.AppendRandomString("vol")
		optional := false
		if secretName != "" {
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
						Optional:   &optional,
					},
				},
			})
		}
		if configMapName != "" {
			volumes = append(volumes, corev1.Volume{
				Name: volumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: configMapName},
						Optional:             &optional,
					},
				},
			})
		}
	}

	container := workloads.NewContainer(containerName, imageName, pullPolicy, nil, envFrom, nil, nil, nil)
	containers := []corev1.Container{container}
	return workloads.NewPodTemplate(containers, volumes, nil, nil, nil)
}

// CheckPodLogsForErrors is a helper to check pod logs for errors
func CheckPodLogsForErrors(client *rancher.Client, clusterID string, podName string, namespace string, errorPattern string, startTime time.Time) error {
	startTimeUTC := startTime.UTC()

	errorRegex := regexp.MustCompile(errorPattern)
	timeRegex := regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)

	var errorMessage string

	kwait.Poll(defaults.TenSecondTimeout, defaults.TwoMinuteTimeout, func() (bool, error) {
		podLogs, err := kubeconfig.GetPodLogs(client, clusterID, podName, namespace, "")
		if err != nil {
			return false, err
		}

		segments := strings.Split(podLogs, "\n")
		for _, segment := range segments {
			timeMatches := timeRegex.FindStringSubmatch(segment)
			if len(timeMatches) > 0 {
				segmentTime, err := time.Parse(timeFormat, timeMatches[0])
				if err != nil {
					continue
				}

				segmentTimeUTC := segmentTime.UTC()
				if segmentTimeUTC.After(startTimeUTC) {
					if matches := errorRegex.FindStringSubmatch(segment); len(matches) > 0 {
						errorMessage = "error logs found in rancher: " + segment
						return true, nil
					}
				}
			}
		}
		return false, nil
	})

	if errorMessage != "" {
		return errors.New(errorMessage)
	}

	return nil
}

// WatchAndWaitPodContainerRunning is a helper to watch and wait all pod containers running
func WatchAndWaitPodContainerRunning(client *rancher.Client, clusterID, namespaceName string, deploymentTemplate *appv1.Deployment) error {
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	namespacedClient := steveclient.SteveType(podSteveType).NamespacedSteveClient(namespaceName)

	backoff := kwait.Backoff{
		Duration: 5 * time.Second,
		Factor:   1,
		Jitter:   0,
		Steps:    10,
	}

	err = kwait.ExponentialBackoff(backoff, func() (finished bool, err error) {
		podsResp, err := namespacedClient.List(nil)
		if err != nil {
			return false, err
		}

		for _, podResp := range podsResp.Data {
			podStatus := &corev1.PodStatus{}
			err = v1.ConvertToK8sType(podResp.Status, podStatus)
			if err != nil {
				return false, err
			}

			for _, containerStatus := range podStatus.ContainerStatuses {
				if containerStatus.State.Running == nil {
					return false, nil
				}
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	return nil
}

// CountPodContainerRunningByImage is a helper to count all pod containers running by image
func CountPodContainerRunningByImage(client *rancher.Client, clusterID, namespaceName string, image string) (int, error) {
	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return 0, err
	}

	podsResp, err := steveclient.SteveType(podSteveType).NamespacedSteveClient(namespaceName).List(nil)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, podResp := range podsResp.Data {
		podStatus := &corev1.PodStatus{}
		err = v1.ConvertToK8sType(podResp.Status, podStatus)
		if err != nil {
			return 0, err
		}
		for _, containerStatus := range podStatus.ContainerStatuses {
			if containerStatus.State.Running != nil && strings.Contains(containerStatus.Image, image) {
				count++
			}
		}
	}
	return count, nil
}

// GetPodByName is a helper to retrieve Pod information by Pod name
func GetPodByName(client *rancher.Client, clusterID, namespaceName, podName string) (*corev1.Pod, error) {
	downstreamContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return nil, err
	}

	updatedPodList, err := downstreamContext.Core.Pod().List(namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + podName,
	})
	if err != nil {
		return nil, err
	}

	if len(updatedPodList.Items) == 0 {
		return nil, fmt.Errorf("deployment %s not found", podName)
	}
	updatedPod := updatedPodList.Items[0]

	return &updatedPod, nil
}

// GetPodNamesFromDeployment is a helper to get names of the pod in a deployment
func GetPodNamesFromDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string) ([]string, error) {
	deploymentList, err := deployments.ListDeployments(client, clusterID, namespaceName, metav1.ListOptions{
		FieldSelector: "metadata.name=" + deploymentName,
	})
	if err != nil {
		return nil, err
	}

	if len(deploymentList.Items) == 0 {
		return nil, fmt.Errorf("deployment %s not found", deploymentName)
	}
	deployment := deploymentList.Items[0]
	selector := deployment.Spec.Selector
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}

	var podNames []string
	downstreamContext, err := client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	if err != nil {
		return nil, err
	}
	pods, err := downstreamContext.Core.Pod().List(namespaceName, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}
