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

// GetProjectList is a helper function that returns all the project in a specific cluster
func GetProjectList(client *rancher.Client, clusterID string) (*management.ProjectCollection, error) {
	var project *management.ProjectCollection

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


	return projectsList, nil
}


// GetProjectList is a helper function that deletes a project in a specific cluster
func DeleteProject(client *rancher.Client, project *management.Project) (error) {

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return  err
	}
	err = adminClient.Management.Project.Delete(project)
	if err != nil {
		return err
	}


	return err
}
