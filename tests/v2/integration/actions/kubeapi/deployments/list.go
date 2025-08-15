package deployments

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	appv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentList is a struct that contains a list of deployments.
type DeploymentList struct {
	Items []appv1.Deployment
}

// ListDeployments is a helper function that uses the dynamic client to list deployments on a namespace for a specific cluster with its list options.
func ListDeployments(client *rancher.Client, clusterID, namespace string, listOpts metav1.ListOptions) (*DeploymentList, error) {
	deploymentList := new(DeploymentList)

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

		deploymentList.Items = append(deploymentList.Items, *newDeployment)
	}

	return deploymentList, nil
}

// Names is a method that accepts DeploymentList as a receiver,
// returns each deployment name in the list as a new slice of strings.
func (list *DeploymentList) Names() []string {
	var deploymentNames []string

	for _, deployment := range list.Items {
		deploymentNames = append(deploymentNames, deployment.Name)
	}

	return deploymentNames
}
