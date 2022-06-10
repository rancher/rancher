package projects

import (
	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
)

// GetProjectByName is a helper function that returns the project by name in a specific cluster
func GetProjectByName(client *rancher.Client, clusterID, projectName string) (*management.Project, error) {
	var project *management.Project

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return project, err
	}

	projectsList, err := adminClient.Management.Project.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return project, err
	}

	for _, p := range projectsList.Data {
		if p.Name == projectName {
			project = &p
		}
	}

	return project, nil
}
