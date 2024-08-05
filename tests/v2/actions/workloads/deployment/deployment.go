package deployment

import (
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/kubeapi/workloads/deployments"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	imageName = "nginx"
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
		)
	}

	createdDeployment, err := deployments.CreateDeployment(client, clusterID, deploymentName, namespaceName, podTemplate, replicas)
	if err != nil {
		return nil, err
	}
	return createdDeployment, nil
}

// UpdateDeployment is a helper to update an existing deployment
func UpdateDeployment(client *rancher.Client, clusterID string, deployment, updates *appv1.Deployment) (*v1.SteveAPIObject, error) {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	deploymentID := deployment.Namespace + "/" + deployment.Name
	deploymentResp, err := steveClient.SteveType(workloads.DeploymentSteveType).ByID(deploymentID)
	if err != nil {
		return nil, err
	}

	updatedDeployment, err := steveClient.SteveType(workloads.DeploymentSteveType).Update(deploymentResp, updates)
	if err != nil {
		return nil, err
	}

	return updatedDeployment, nil
}

// DeleteDeployment is a helper to delete a deployment
func DeleteDeployment(client *rancher.Client, clusterID string, deployment *appv1.Deployment) error {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	deploymentID := deployment.Namespace + "/" + deployment.Name
	deploymentResp, err := steveClient.SteveType(workloads.DeploymentSteveType).ByID(deploymentID)
	if err != nil {
		return err
	}

	err = steveClient.SteveType(workloads.DeploymentSteveType).Delete(deploymentResp)
	if err != nil {
		return err
	}

	return nil
}
