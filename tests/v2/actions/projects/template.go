package projects

import (
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
)

const projectName = "testproject-"

// NewProjectConfig is a constructor that creates a project template
func NewProjectConfig(clusterID string) *management.Project {
	return &management.Project{
		ClusterID: clusterID,
		Name:      namegen.AppendRandomString(projectName),
	}
}
