package tokenregistration

import (
	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
)

type ClusterRegistrationToken struct {
	v3.ClusterRegistrationTokenOperations
}

func NewClusterRegistrationToken(client *v3.Client) *ClusterRegistrationToken {
	return &ClusterRegistrationToken{
		client.ClusterRegistrationToken,
	}
}

// CreateRegistrationToken is function that creates a ClusterRegistrationToken using a Client object with a specified *v3.ClusterRegistrationToken
func (c *ClusterRegistrationToken) CreateRegistrationToken(clusterRegistrationToken *v3.ClusterRegistrationToken) (*v3.ClusterRegistrationToken, error) {
	return c.Create(clusterRegistrationToken)
}

// GetTokenRegistration is function thatm gets a specific ClusterRegistrationToken using a Client object with a specified clusterStatusName and token name
func (c *ClusterRegistrationToken) GetRegistrationToken(clusterId string) (*v3.ClusterRegistrationToken, error) {
	collection, err := c.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterId,
		},
	})

	if err != nil {
		return nil, err
	}

	registrationToken := collection.Data[0]

	return &registrationToken, nil
}
