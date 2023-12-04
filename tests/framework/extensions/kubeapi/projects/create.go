package projects

import (
	"context"

	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateProject is a helper function that uses the dynamic client to create a project in a specific cluster.
func CreateProject(client *rancher.Client, project *v3.Project) (*v3.Project, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(localCluster)
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
