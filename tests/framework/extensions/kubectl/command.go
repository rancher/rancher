package kubectl

import (
	"fmt"
	"net"
	"strings"

	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	corev1 "k8s.io/api/core/v1"

	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"

	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
)

const volumeName = "config"

// Command is a helper function that call a helper to create a Job and executing kubectl by copying the yaml
// in the pod. It returns the output from the pod logs.
func Command(client *rancher.Client, cluster *apisV1.Cluster, yamlContent *management.ImportClusterYamlInput, clusterID string, command []string) (string, error) {
	var user int64
	var group int64
	imageSetting, err := client.Management.Setting.ByID(rancherShellSettingID)
	if err != nil {
		return "", err
	}

	id := namegen.RandStringLower(6)
	jobName := fmt.Sprintf("%v-%v", JobName, id)

	initVolumeMount := []corev1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: "/config",
		},
	}

	volumeMount := []corev1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: "/root/.kube",
		},
	}

	securityContext := &corev1.SecurityContext{
		RunAsUser:  &user,
		RunAsGroup: &group,
	}

	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	jobTemplate := workloads.NewJobTemplate(jobName, Namespace)

	if yamlContent != nil {
		initContainerCommand := []string{"sh", "-c", fmt.Sprintf("echo \"%s\" > /config/my-pod.yaml", strings.ReplaceAll(yamlContent.YAML, "\"", "\\\""))}
		initContainer := workloads.NewContainer("copy-yaml", imageSetting.Value, corev1.PullAlways, initVolumeMount, nil, initContainerCommand, nil, nil)
		jobTemplate.Spec.Template.Spec.InitContainers = append(jobTemplate.Spec.Template.Spec.InitContainers, initContainer)
	}

	container := workloads.NewContainer(jobName, imageSetting.Value, corev1.PullAlways, volumeMount, nil, command, securityContext, nil)
	jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, container)
	jobTemplate.Spec.Template.Spec.Volumes = volumes
	err = CreateJobAndRunKubectlCommands(clusterID, jobName, jobTemplate, client)
	if err, ok := err.(net.Error); ok && !err.Timeout() {
		return "", err
	}

	steveClient := client.Steve
	pods, err := steveClient.SteveType(pods.PodResourceSteveType).NamespacedSteveClient(Namespace).List(nil)
	if err != nil {
		return "", err
	}

	var podName string
	for _, pod := range pods.Data {
		if strings.Contains(pod.Name, id) {
			podName = pod.Name
			break
		}
	}
	podLogs, err := kubeconfig.GetPodLogs(client, clusterID, podName, Namespace)
	if err != nil {
		return "", err
	}

	return podLogs, nil
}
