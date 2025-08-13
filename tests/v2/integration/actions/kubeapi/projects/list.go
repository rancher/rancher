package projects

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/pkg/api/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListProjects is a helper function that uses the dynamic client to list projects in a cluster.
func ListProjects(client *rancher.Client, namespace string, listOpt metav1.ListOptions) (*v3.ProjectList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(ProjectGroupVersionResource).Namespace(namespace).List(context.Background(), listOpt)
	if err != nil {
		return nil, err
	}

	projectList := new(v3.ProjectList)
	for _, unstructuredProjects := range unstructuredList.Items {
		project := &v3.Project{}
		err := scheme.Scheme.Convert(&unstructuredProjects, project, unstructuredProjects.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		projectList.Items = append(projectList.Items, *project)
	}

	return projectList, nil
}
