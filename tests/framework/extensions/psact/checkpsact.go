package psact

import (
	"fmt"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
)

// CheckPSACT checks to see if PSACT is enabled or not in the cluster.
func CheckPSACT(client *rancher.Client, clusterName string) error {
	clusterID, err := clusters.GetClusterIDByName(client, clusterName)
	if err != nil {
		return err
	}

	cluster, err := client.Management.Cluster.ByID(clusterID)
	if err != nil {
		return err
	}

	if cluster.DefaultPodSecurityAdmissionConfigurationTemplateName == "" {
		return fmt.Errorf("error: PSACT is not defined in this cluster")
	}

	return nil
}
