package projects

import (
	"context"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/unstructured"
	"github.com/rancher/shepherd/pkg/api/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateProject is a helper function that uses the dynamic client to update a project in a cluster.
func UpdateProject(client *rancher.Client, existingProject *v3.Project, updatedProject *v3.Project) (*v3.Project, error) {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return nil, err
	}

	projectResource := dynamicClient.Resource(ProjectGroupVersionResource).Namespace(existingProject.Namespace)

	projectUnstructured, err := projectResource.Get(context.TODO(), existingProject.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	currentProject := &v3.Project{}
	err = scheme.Scheme.Convert(projectUnstructured, currentProject, projectUnstructured.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	updatedProject.ObjectMeta.ResourceVersion = currentProject.ObjectMeta.ResourceVersion

	unstructuredResp, err := projectResource.Update(context.TODO(), unstructured.MustToUnstructured(updatedProject), metav1.UpdateOptions{})
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
