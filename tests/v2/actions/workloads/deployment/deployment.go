package deployment

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/kubeapi/workloads/deployments"
	"github.com/rancher/rancher/tests/v2/actions/workloads/pods"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/extensions/workloads"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/wrangler"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	active              = "active"
	defaultNamespace    = "default"
	port                = "port"
	DeploymentSteveType = "apps.deployment"
	nginxImageName      = "public.ecr.aws/docker/library/nginx"
	ubuntuImageName     = "ubuntu"
	redisImageName      = "public.ecr.aws/docker/library/redis"
	podSteveType        = "pod"
)

// CreateDeployment is a helper to create a deployment with or without a secret/configmap
func CreateDeployment(client *rancher.Client, clusterID, namespaceName string, replicaCount int, secretName, configMapName string, useEnvVars, useVolumes, isRegistrySecret, watchDeployment bool) (*appv1.Deployment, error) {
	deploymentName := namegen.AppendRandomString("testdeployment")
	containerName := namegen.AppendRandomString("testcontainer")
	pullPolicy := corev1.PullAlways
	replicas := int32(replicaCount)

	var podTemplate corev1.PodTemplateSpec

	if secretName != "" || configMapName != "" {
		if isRegistrySecret {
			podTemplate = pods.NewPodTemplateWithConfig(secretName, configMapName, useEnvVars, useVolumes)
			podTemplate.Spec.ImagePullSecrets = append(podTemplate.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
		} else {
			podTemplate = pods.NewPodTemplateWithConfig(secretName, configMapName, useEnvVars, useVolumes)
		}
	} else {
		containerTemplate := workloads.NewContainer(
			containerName,
			nginxImageName,
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
			nil,
		)
	}

	createdDeployment, err := deployments.CreateDeployment(client, clusterID, deploymentName, namespaceName, podTemplate, replicas)
	if err != nil {
		return nil, err
	}

	if watchDeployment {
		err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
			FieldSelector: "metadata.name=" + createdDeployment.Name,
		})
	}

	return createdDeployment, err
}

// UpdateDeployment is a helper to update deployments
func UpdateDeployment(client *rancher.Client, clusterID, namespaceName string, deployment *appv1.Deployment, watchDeployment bool) (*appv1.Deployment, error) {
	var wranglerContext *wrangler.Context
	var err error

	wranglerContext = client.WranglerContext
	if clusterID != "local" {
		wranglerContext, err = client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
		if err != nil {
			return nil, err
		}
	}

	latestDeployment, err := wranglerContext.Apps.Deployment().Get(namespaceName, deployment.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	deployment.ResourceVersion = latestDeployment.ResourceVersion

	updatedDeployment, err := wranglerContext.Apps.Deployment().Update(deployment)
	if err != nil {
		return nil, err
	}

	if watchDeployment {
		err = charts.WatchAndWaitDeployments(client, clusterID, namespaceName, metav1.ListOptions{
			FieldSelector: "metadata.name=" + updatedDeployment.Name,
		})
	}

	return updatedDeployment, err
}

// DeleteDeployment is a helper to delete a deployment
func DeleteDeployment(client *rancher.Client, clusterID string, deployment *appv1.Deployment) error {
	steveClient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	deploymentID := deployment.Namespace + "/" + deployment.Name
	deploymentResp, err := steveClient.SteveType(DeploymentSteveType).ByID(deploymentID)
	if err != nil {
		return err
	}

	err = steveClient.SteveType(DeploymentSteveType).Delete(deploymentResp)
	if err != nil {
		return err
	}

	return nil
}

func RollbackDeployment(client *rancher.Client, clusterID, namespaceName string, deploymentName string, revision int) (string, error) {
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
