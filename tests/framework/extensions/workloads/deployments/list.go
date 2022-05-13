package deployments

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListDeployments is a helper function that uses the dynamic client to list deployments on a namespace for a specific cluster with its list options.
func ListDeployments(client *rancher.Client, clusterID, namespace string, listOpts metav1.ListOptions) ([]appv1.Deployment, error) {
	var deploymentList []appv1.Deployment

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}
	deploymentResource := dynamicClient.Resource(DeploymentGroupVersionResource).Namespace(namespace)
	deployments, err := deploymentResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, err
	}

	for _, unstructuredDeployment := range deployments.Items {
		newDeployment := &appv1.Deployment{}
		err := scheme.Scheme.Convert(&unstructuredDeployment, newDeployment, unstructuredDeployment.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		deploymentList = append(deploymentList, *newDeployment)
	}

	return deploymentList, nil
}
