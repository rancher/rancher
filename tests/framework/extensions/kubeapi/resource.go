package kubeapi

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func ResourceForClient(client *rancher.Client, clusterName, namespace string, resource schema.GroupVersionResource) (dynamic.ResourceInterface, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(resource).Namespace(namespace), nil
}
