package customresourcedefinitions

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// deletes a single custom resource definition by name
func DeleteCustomResourceDefinition(client *rancher.Client, clusterID string, namespace string, name string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	customResourceDefinitionResource := dynamicClient.Resource(CustomResourceDefinitions).Namespace(namespace)

	err = customResourceDefinitionResource.Delete(context.TODO(), name, metav1.DeleteOptions{})

	return err
}

// deletes a list of custom resource definitions by name
func BatchDeleteCustomResourceDefinition(client *rancher.Client, clusterID string, namespace string, list []string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return err
	}

	customResourceDefinitionResource := dynamicClient.Resource(CustomResourceDefinitions).Namespace(namespace)

	var errs error
	for _, crd := range list {
		err = customResourceDefinitionResource.Delete(context.TODO(), crd, metav1.DeleteOptions{})
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
	}

	return errs
}
