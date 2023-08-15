package workloads

import (
	"time"

	steveV1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	appv1 "k8s.io/api/apps/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// VerifyDeployment waits for a deployment to be ready
func VerifyDeployment(steveClient *steveV1.Client, deployment *steveV1.SteveAPIObject) error {
	err := kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		if err != nil {
			return false, nil
		}
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
