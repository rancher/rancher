package projects

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateProject is a helper function that uses the dynamic client to create a project in a cluster.
func CreateProject(client *rancher.Client, project *v3.Project) (*v3.Project, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	projectResource := dynamicClient.Resource(ProjectGroupVersionResource).Namespace(project.Namespace)
	unstructuredResp, err := projectResource.Create(context.TODO(), unstructured.MustToUnstructured(project), metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	newProject := &v3.Project{}
	err = scheme.Scheme.Convert(unstructuredResp, newProject, unstructuredResp.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	return newProject, nil
}
