package secrets

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretList is a struct that contains a list of secrets.
type SecretList struct {
	Items []corev1.Secret
}

// ListSecrets is a helper function that uses the dynamic client to list secrets in a cluster with its list options.
func ListSecrets(client *rancher.Client, clusterID, namespace string, listOpts metav1.ListOptions) (*SecretList, error) {
	secretList := new(SecretList)

	dynamicClient, err := client.GetDownStreamClusterClient(clusterID)
	if err != nil {
		return nil, err
	}

	secretResource := dynamicClient.Resource(SecretGroupVersionResource).Namespace(namespace)
	secrets, err := secretResource.List(context.TODO(), listOpts)
	if err != nil {
		return nil, err
	}

	for _, unstructuredSecret := range secrets.Items {
		newSecret := &corev1.Secret{}

		err := scheme.Scheme.Convert(&unstructuredSecret, newSecret, unstructuredSecret.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		secretList.Items = append(secretList.Items, *newSecret)
	}

	return secretList, nil
}

// Names is a method that accepts SecretList as a receiver,
// returns each secret name in the list as a new slice of strings.
func (list *SecretList) Names() []string {
	var secretNames []string

	for _, secret := range list.Items {
		secretNames = append(secretNames, secret.Name)
	}

	return secretNames
}
