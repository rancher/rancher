package psact

import (
	"fmt"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	actionsworkloads "github.com/rancher/rancher/tests/v2/actions/workloads"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/workloads"
	namegenerator "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/sirupsen/logrus"
	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const (
	containerName     = "nginx"
	deploymentName    = "nginx"
	imageName         = "nginx"
	namespace         = "default"
	workload          = "workload"
	podFailureMessage = `forbidden: violates PodSecurity "restricted:latest"`
)

// CreateTestDeployment will create an nginx deployment into the default namespace. If the PSACT value is rancher-privileged, then the
// deployment should successfully create. If the PSACT value is rancher-unprivileged, then the deployment should fail to create.
func CreateNginxDeployment(client *rancher.Client, clusterID string, psact string) error {
	labels := map[string]string{}
	labels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", namespace, workload)

	containerTemplate := workloads.NewContainer(containerName, imageName, v1.PullAlways, []v1.VolumeMount{}, []v1.EnvFromSource{}, nil, nil, nil)
	podTemplate := workloads.NewPodTemplate([]v1.Container{containerTemplate}, []v1.Volume{}, []v1.LocalObjectReference{}, labels, nil)
	deploymentTemplate := workloads.NewDeploymentTemplate(deploymentName, namespace, podTemplate, true, labels)

	steveclient, err := client.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return err
	}

	// If the deployment already exists, then create a new deployment with a different name to avoid a naming conflict.
	if _, err := steveclient.SteveType(actionsworkloads.DeploymentSteveType).ByID(deploymentTemplate.Namespace + "/" + deploymentTemplate.Name); err == nil {
		deploymentTemplate.Name = deploymentTemplate.Name + "-" + namegenerator.RandStringLower(5)
	}

	logrus.Infof("Creating deployment %s", deploymentTemplate.Name)
	_, err = steveclient.SteveType(actionsworkloads.DeploymentSteveType).Create(deploymentTemplate)
	if err != nil {
		return err
	}

	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		steveclient, err := client.Steve.ProxyDownstream(clusterID)
		if err != nil {
			return false, err
		}

		deploymentResp, err := steveclient.SteveType(actionsworkloads.DeploymentSteveType).ByID(deploymentTemplate.Namespace + "/" + deploymentTemplate.Name)
		if err != nil {
			// We don't want to return the error so we don't exit the poll too soon.
			// There could be delay of when the deployment is created.
			return false, nil
		}

		deployment := &appv1.Deployment{}
		err = steveV1.ConvertToK8sType(deploymentResp.JSONResp, deployment)
		if err != nil {
			return false, err
		}

		if psact == string(provisioninginput.RancherRestricted) {
			for _, condition := range deployment.Status.Conditions {
				if strings.Contains(condition.Message, podFailureMessage) {
					logrus.Infof("Deployment %s failed to create; this is expected for %s!", deployment.Name, psact)
					return true, nil
				}
			}
		} else if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
			logrus.Infof("Deployment %s successfully created; this is expected for %s!", deployment.Name, psact)
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}

	deploymentResp, err := steveclient.SteveType(actionsworkloads.DeploymentSteveType).ByID(deploymentTemplate.Namespace + "/" + deploymentTemplate.Name)
	if err != nil {
		return err
	}

	logrus.Infof("Deleting deployment %s", deploymentResp.Name)
	err = steveclient.SteveType(actionsworkloads.DeploymentSteveType).Delete(deploymentResp)
	if err != nil {
		return err
	}

	return nil
}
