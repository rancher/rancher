package bundledclusters

import (
	"github.com/pkg/errors"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	available "github.com/rancher/rancher/tests/framework/extensions/clusters/kubernetesversions"
)

// ListAvailable is a method of BundledCluster that uses list available functions
// depending on cluster's provider information. Returns versions as slice of strings
// and error if any.
func (bc *BundledCluster) ListAvailableVersions(client *rancher.Client) (versions []string, err error) {
	switch bc.Meta.Provider {
	case clusters.KubernetesProviderRKE:
		if bc.Meta.IsImported {
			versions, err = available.ListRKE1ImportedAvailableVersions(client, bc.V3)
			if err != nil {
				return
			}
		} else {
			versions, err = available.ListRKE1AvailableVersions(client, bc.V3)
			if err != nil {
				return
			}
		}
	case clusters.KubernetesProviderRKE2:
		if bc.Meta.IsImported {
			versions, err = available.ListRKE2AvailableVersions(client, bc.V1)
			if err != nil {
				return
			}
		} else {
			versions, err = available.ListNormanRKE2AvailableVersions(client, bc.V3)
			if err != nil {
				return
			}
		}
	case clusters.KubernetesProviderK3S:
		if bc.Meta.IsImported {
			versions, err = available.ListK3SAvailableVersions(client, bc.V1)
			if err != nil {
				return
			}
		} else {
			versions, err = available.ListNormanK3SAvailableVersions(client, bc.V3)
			if err != nil {
				return
			}
		}
	case clusters.KubernetesProviderGKE:
		versions, err = available.ListGKEAvailableVersions(client, bc.V3)
		if err != nil {
			return
		}
	case clusters.KubernetesProviderAKS:
		versions, err = available.ListAKSAvailableVersions(client, bc.V3)
		if err != nil {
			return
		}
	case clusters.KubernetesProviderEKS:
		versions, err = available.ListEKSAvailableVersions(client, bc.V3)
		if err != nil {
			return
		}
	default:
		return nil, errors.Wrap(err, "list available versions failed")
	}

	return
}
