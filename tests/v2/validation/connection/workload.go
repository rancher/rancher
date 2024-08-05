package connection

import (
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/workloads"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultNamespace = "default"
)

func createDeployment(steveclient *steveV1.Client, containersTemplate []corev1.Container, containerName string) (*steveV1.SteveAPIObject, error) {
	podTemplate := workloads.NewPodTemplate(containersTemplate, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil)
	deployment := workloads.NewDeploymentTemplate(containerName, defaultNamespace, podTemplate, true, nil)
	deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}

	return deploymentResp, err
}
