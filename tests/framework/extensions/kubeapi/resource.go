package kubeapi

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ResourceForClient is a helper function that creates a dynamic client and instantiates a subsequent resource interface
// in the requested cluster and namespace for said resource.
func ResourceForClient(client *rancher.Client, clusterName, namespace string, resource schema.GroupVersionResource) (dynamic.ResourceInterface, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(resource).Namespace(namespace), nil
}
