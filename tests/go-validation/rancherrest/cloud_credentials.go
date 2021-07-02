package rancherrest

import (
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var DOAccessToken string = os.Getenv("DO_ACCESSKEY")

// NewDigitalOceanCloudCredential is a method that creates a *v1.Secret specific to a cloud credential
// for Digital Ocean
func NewDigitalOceanCloudCredential(cloudCredentialName, description, namespace string) *v1.Secret {
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cloudCredentialName,
			Namespace: namespace,
			Annotations: map[string]string{
				"field.cattle.io/description":   description,
				"provisioning.cattle.io/driver": "digitalocean",
			},
		},
		Data: map[string][]byte{
			"accessToken": []byte(DOAccessToken),
		},
		Type: "provisioning.cattle.io/cloud-credential",
	}
}

// CreateCloudCredential is function that creates a cloud credential using a Client object with a specified v1.Secret
func (c *Client) CreateCloudCredential(secret *v1.Secret) (*v1.Secret, error) {
	returnedSecret, err := c.Context.Core.Secret().Create(secret)
	return returnedSecret, err
}

// UpdateCloudCredential is function that updates a cloud credential using a Client object
func (c *Client) UpdateCloudCredential(secret *v1.Secret) (*v1.Secret, error) {
	returnedSecret, err := c.Context.Core.Secret().Update(secret)
	return returnedSecret, err
}

// DeleteCloudCredential is function that deletes a cloud credential using a Client object
func (c *Client) DeleteCloudCredential(secret *v1.Secret) error {
	nameSpace := secret.Namespace
	name := secret.Name
	return c.Context.Core.Secret().Delete(nameSpace, name, &metav1.DeleteOptions{})
}

// GetCloudCredential is function that gets a cloud credential using a Client object
func (c *Client) GetCloudCredential(secret *v1.Secret) (*v1.Secret, error) {
	nameSpace := secret.Namespace
	name := secret.Name

	returnedSecret, err := c.Context.Core.Secret().Get(nameSpace, name, metav1.GetOptions{})
	return returnedSecret, err
}

// ListCloudCredential is function that lists cloud credentials from a specific namespace using a Client object
func (c *Client) ListCloudCredential(nameSpace string) (*v1.SecretList, error) {

	returnedSecret, err := c.Context.Core.Secret().List(nameSpace, metav1.ListOptions{})
	return returnedSecret, err
}
