package tokenregistration

import (
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// GetRegistrationToken is function that gets a specific ClusterRegistrationToken using a Client object with a specified clusterStatusName and token name.
// It is done using a poll wait to make sure the tokens have been created by rancher.
func GetRegistrationToken(client *rancher.Client, clusterID string) (*management.ClusterRegistrationToken, error) {
	var clusterRegistrationTokens []management.ClusterRegistrationToken

	kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		collection, err := client.Management.ClusterRegistrationToken.List(&types.ListOpts{
			Filters: map[string]interface{}{
				"clusterId": clusterID,
			},
		})

		if err != nil {
			return false, err
		}

		if len(collection.Data) == 0 {
			return false, err
		}

		clusterRegistrationTokens = collection.Data
		return true, nil
	})

	registrationToken := clusterRegistrationTokens[0]

	return &registrationToken, nil
}
