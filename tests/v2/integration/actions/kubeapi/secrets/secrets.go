package secrets

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretGroupVersionResource is the required Group Version Resource for accessing secrets in a cluster,
// using the dynamic client.
var SecretGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "secrets",
}

// GetSecretByName is a helper function that uses the dynamic client to get a specific secret on a namespace for a specific cluster.
func GetSecretByName(client *rancher.Client, clusterID, namespace, secretName string, getOpts metav1.GetOptions) (*coreV1.Secret, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	secretResource := dynamicClient.Resource(SecretGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := secretResource.Get(context.TODO(), secretName, getOpts)
	if err != nil {
		return nil, err
	}

	newSecret := &coreV1.Secret{}
	err = scheme.Scheme.Convert(unstructuredResp, newSecret, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}
	return newSecret, nil
}
