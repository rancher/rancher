package cloudcredentials

import (
	"os"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

var DOAccessToken string = os.Getenv("DO_ACCESSKEY")

type CloudCredential struct {
	v3.CloudCredentialOperations
}

func NewCloudCredentialConfig(cloudCredentialName, description, driverType, namespace string) *v3.CloudCredential {
	cloudCredential := &v3.CloudCredential{
		Name:        cloudCredentialName,
		Description: description,
	}
	switch driverType {
	case "digitalocean":
		doCloudCredSpec := &v3.DigitalOceanCredentialConfig{
			AccessToken: DOAccessToken,
		}
		cloudCredential.DigitalOceanCredentialConfig = doCloudCredSpec
	}
	return cloudCredential
}

// NewCloudCredential creates a CloudCredential object
func NewCloudCredential(client *v3.Client) *CloudCredential {
	return &CloudCredential{
		client.CloudCredential,
	}
}

// CreateCloudCredential is function that creates a cloud credential using a CloudCredential object with a specified *v3.CloudCredential
func (c *CloudCredential) CreateCloudCredential(cloudCred *v3.CloudCredential) (*v3.CloudCredential, error) {
	returnedCredential, err := c.Create(cloudCred)
	return returnedCredential, err
}

// UpdateCloudCredential is function that updates a cloud credential using a CloudCredential object and an updates interface
func (c *CloudCredential) UpdateCloudCredential(existingCloudCred *v3.CloudCredential, updates interface{}) (*v3.CloudCredential, error) {
	returnedCredential, err := c.Update(existingCloudCred, updates)
	return returnedCredential, err
}

// DeleteCloudCredential is function that deletes a cloud credential using a CloudCredential object
func (c *CloudCredential) DeleteCloudCredential(cloudCred *v3.CloudCredential) error {
	return c.Delete(cloudCred)
}

// GetCloudCredential is function that gets a cloud credential using a CloudCredential object
func (c *CloudCredential) GetCloudCredential(cloudCred *v3.CloudCredential) (*v3.CloudCredential, error) {
	returnedSecret, err := c.ByID(cloudCred.ID)
	return returnedSecret, err
}

// ListCloudCredential is function that lists cloud credentials
func (c *CloudCredential) ListCloudCredential(nameSpace string) (*v3.CloudCredentialCollection, error) {
	returnedSecret, err := c.List(&types.ListOpts{})
	return returnedSecret, err
}
