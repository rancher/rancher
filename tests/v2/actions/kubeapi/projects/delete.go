package projects

import (
	"context"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// DeleteProject is a helper function that uses the dynamic client to delete a Project from a cluster.
func DeleteProject(client *rancher.Client, projectNamespace string, projectName string) error {
	dynamicClient, err := client.GetDownStreamClusterClient(LocalCluster)
	if err != nil {
		return err
	}

	projectResource := dynamicClient.Resource(ProjectGroupVersionResource).Namespace(projectNamespace)

	err = projectResource.Delete(context.TODO(), projectName, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	err = kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, err error) {
		projectList, err := ListProjects(client, projectNamespace, metav1.ListOptions{
			FieldSelector: "metadata.name=" + projectName,
		})

		if err != nil {
			return false, err
		}

		if len(projectList.Items) == 0 {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return err
	}

	return nil
}
