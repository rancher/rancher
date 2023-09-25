package projects

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const projectName = "testproject-"

// NewProjectConfig is a constructor that creates a project template
func NewProjectConfig(clusterID string) *management.Project {
	return &management.Project{
		ClusterID: clusterID,
		Name:      namegen.AppendRandomString(projectName),
	}
}
