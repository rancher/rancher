package workloads

import (
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/workloads"
	deployment "github.com/rancher/shepherd/extensions/workloads/deployment"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultNamespace = "default"
)

func CreateDeployment(steveclient *steveV1.Client, containersTemplate []corev1.Container, containerName string) (*steveV1.SteveAPIObject, error) {
	podTemplate := workloads.NewPodTemplate(containersTemplate, []corev1.Volume{}, []corev1.LocalObjectReference{}, nil)
	deploymentTemplate := workloads.NewDeploymentTemplate(containerName, defaultNamespace, podTemplate, true, nil)
	return deployment.CreateDeployment(steveclient, containerName, deploymentTemplate)
}
