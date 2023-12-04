package projects

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListProjects is a helper function that uses the dynamic client to list projects in a specific cluster.
func ListProjects(client *rancher.Client, namespace string, listOpt metav1.ListOptions) (*v3.ProjectList, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(localCluster)
	if err != nil {
		return nil, err
	}

	unstructuredList, err := dynamicClient.Resource(ProjectGroupVersionResource).Namespace(namespace).List(context.Background(), listOpt)
	if err != nil {
		return nil, err
	}

	projectList := new(v3.ProjectList)
	for _, unstructuredCRTB := range unstructuredList.Items {
		project := &v3.Project{}
		err := scheme.Scheme.Convert(&unstructuredCRTB, project, unstructuredCRTB.GroupVersionKind())
		if err != nil {
			return nil, err
		}

		projectList.Items = append(projectList.Items, *project)
	}

	return projectList, nil
}
