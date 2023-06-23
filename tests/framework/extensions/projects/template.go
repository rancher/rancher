package projects

import (
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

// NewProjectConfig is a contructor that creates a container for a pod template i.e. corev1.PodTemplateSpec
func NewProjectConfig(clusterID string) *management.Project {
	return &management.Project{
		ClusterID: clusterID,
		Name:      namegen.AppendRandomString("testproject-"),
	}
}
