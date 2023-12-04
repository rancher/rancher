package projects

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteProject is a helper function that uses the dynamic client to delete a Project from a specific cluster.
func DeleteProject(client *rancher.Client, project *v3.Project) error {
	dynamicClient, err := client.GetDownStreamClusterClient(localCluster)
	if err != nil {
		return err
	}

	projectResource := dynamicClient.Resource(ProjectGroupVersionResource).Namespace(project.Namespace)

	err = projectResource.Delete(context.TODO(), project.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
