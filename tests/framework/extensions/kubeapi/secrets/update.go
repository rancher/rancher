package secrets

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateSecret is a helper function that uses the dynamic client to update a secret in a specific cluster.
func UpdateSecret(client *rancher.Client, clusterID string, updatedSecret *coreV1.Secret) (*coreV1.Secret, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	secretResource := dynamicClient.Resource(SecretGroupVersionResource).Namespace(updatedSecret.Namespace)

	secretUnstructured, err := secretResource.Get(context.TODO(), updatedSecret.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	currentSecret := &coreV1.Secret{}
	err = scheme.Scheme.Convert(secretUnstructured, currentSecret, secretUnstructured.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	updatedSecret.ObjectMeta.ResourceVersion = currentSecret.ObjectMeta.ResourceVersion

	unstructuredResp, err := secretResource.Update(context.TODO(), unstructured.MustToUnstructured(updatedSecret), metav1.UpdateOptions{})
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
