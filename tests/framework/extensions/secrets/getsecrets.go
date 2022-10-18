package secrets

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetSecret is a helper function that uses the dynamic client to get a specific secret in a namespace for a specific cluster.
func GetSecret(client *rancher.Client, clusterID, namespace, secretName string, getOpts metav1.GetOptions) (*coreV1.Secret, error) {
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
