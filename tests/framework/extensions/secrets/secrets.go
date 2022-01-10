package secrets

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var SecretGroupVersionResource = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "secrets",
}

// CreateSecret is a helper function that uses the dynamic client to create a secret on a namespace for a specific cluster.
// It registers a delete fuction.
func CreateSecret(client *rancher.Client, secret *coreV1.Secret, clusterName, namespace string) (*coreV1.Secret, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterName)
	if err != nil {
		return nil, err
	}

	secretResource := dynamicClient.Resource(SecretGroupVersionResource).Namespace(namespace)

	unstructuredResp, err := secretResource.Create(context.TODO(), unstructured.MustToUnstructured(secret), metav1.CreateOptions{})
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
