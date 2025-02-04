package deployments

import (
	"context"

	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	"github.com/rancher/shepherd/pkg/wait"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

// DeploymentGroupVersionResource is the required Group Version Resource for accessing deployments in a cluster,
// using the dynamic client.
var DeploymentGroupVersionResource = schema.GroupVersionResource{
	Group:    "apps",
	Version:  "v1",
	Resource: "deployments",
}

// WatchAndWaitDeployments is a helper function that watches the deployments
// sequentially in a specific namespace and waits until number of expected replicas is equal to number of available replicas.
func WatchAndWaitDeployments(client *rancher.Client, clusterID, namespace string, listOptions metav1.ListOptions) error {
	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return err
	}
	adminDynamicClient, err := adminClient.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	adminDeploymentResource := adminDynamicClient.Resource(DeploymentGroupVersionResource).Namespace(namespace)

	deployments, err := adminDeploymentResource.List(context.TODO(), listOptions)
	if err != nil {
		return err
	}

	var deploymentList []appv1.Deployment

	for _, unstructuredDeployment := range deployments.Items {
		newDeployment := &appv1.Deployment{}
		err := scheme.Scheme.Convert(&unstructuredDeployment, newDeployment, unstructuredDeployment.GroupVersionKind())
		if err != nil {
			return err
		}

		deploymentList = append(deploymentList, *newDeployment)
	}

	for _, deployment := range deploymentList {
		watchAppInterface, err := adminDeploymentResource.Watch(context.TODO(), metav1.ListOptions{
			FieldSelector:  "metadata.name=" + deployment.Name,
			TimeoutSeconds: &defaults.WatchTimeoutSeconds,
		})
		if err != nil {
			return err
		}

		wait.WatchWait(watchAppInterface, func(event watch.Event) (ready bool, err error) {
			deploymentsUnstructured := event.Object.(*unstructured.Unstructured)
			deployment := &appv1.Deployment{}

			err = scheme.Scheme.Convert(deploymentsUnstructured, deployment, deploymentsUnstructured.GroupVersionKind())
			if err != nil {
				return false, err
			}

			if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
				return true, nil
			}
			return false, nil
		})
	}

	return nil
}
