package kubectl

import (
	"fmt"
	"strings"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/kubeconfig"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	corev1 "k8s.io/api/core/v1"
)

// Explain is a helper function that creates a Job by calling the helper CreateJobAndRunKubectlCommands and executing kubectl explain in the pod of the Job
// returns the output from the pod logs.
func Explain(client *rancher.Client, cluster *apisV1.Cluster, cmd, clusterID string) (string, error) {
	jobName := JobName + "-explain"

	imageSetting, err := client.Management.Setting.ByID(rancherShellSettingID)
	if err != nil {
		return "", err
	}

	jobTemplate := workloads.NewJobTemplate(jobName, Namespace)
	args := []string{
		fmt.Sprintf("kubectl explain %s", cmd),
	}
	command := []string{"/bin/sh", "-c"}
	securityContext := &corev1.SecurityContext{
		RunAsUser:  &user,
		RunAsGroup: &group,
	}
	volumeMount := []corev1.VolumeMount{
		{Name: "config", MountPath: "/root/.kube/"},
	}
	container := workloads.NewContainer(jobName, imageSetting.Value, corev1.PullAlways, volumeMount, nil, command, securityContext, args)
	jobTemplate.Spec.Template.Spec.Containers = append(jobTemplate.Spec.Template.Spec.Containers, container)

	err = CreateJobAndRunKubectlCommands(clusterID, jobName, jobTemplate, client)
	if err != nil {
		return "", err
	}

	steveClient := client.Steve
	pods, err := steveClient.SteveType(pods.PodResourceSteveType).NamespacedSteveClient(Namespace).List(nil)
	if err != nil {
		return "", err
	}

	var podName string
	for _, pod := range pods.Data {
		if strings.Contains(pod.Name, jobName) {
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
