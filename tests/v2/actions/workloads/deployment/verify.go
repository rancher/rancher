package deployment

import (
	"context"
	"time"

	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/defaults"
	appv1 "k8s.io/api/apps/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// VerifyDeployment waits for a deployment to be ready in the downstream cluster
func VerifyDeployment(steveClient *steveV1.Client, deployment *steveV1.SteveAPIObject) error {
	err := kwait.PollUntilContextTimeout(context.TODO(), 500*time.Millisecond, defaults.FiveMinuteTimeout, true, func(ctx context.Context) (done bool, err error) {
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
