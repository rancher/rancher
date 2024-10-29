package projects

import (
	"sort"
	"strings"

	"github.com/rancher/norman/types"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	projectsapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/defaults"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

// GetProjectByName is a helper function that returns the project by name in a specific cluster.
func GetProjectByName(client *rancher.Client, clusterID, projectName string) (*management.Project, error) {
	var project *management.Project

	adminClient, err := rancher.NewClient(client.RancherConfig.AdminToken, client.Session)
	if err != nil {
		return project, err
	}

	projectsList, err := adminClient.Management.Project.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return project, err
	}

	for i, p := range projectsList.Data {
		if p.Name == projectName {
			project = &projectsList.Data[i]
			break
		}
	}

	return project, nil
}

// GetProjectList is a helper function that returns all the project in a specific cluster
func GetProjectList(client *rancher.Client, clusterID string) (*management.ProjectCollection, error) {
	var projectsList *management.ProjectCollection

	projectsList, err := client.Management.Project.List(&types.ListOpts{
		Filters: map[string]interface{}{
			"clusterId": clusterID,
		},
	})
	if err != nil {
		return projectsList, err
	}

	return projectsList, nil
}

// ListProjectNames is a helper which returns a sorted list of project names
func ListProjectNames(client *rancher.Client, clusterID string) ([]string, error) {
	projectList, err := GetProjectList(client, clusterID)
	if err != nil {
		return nil, err
	}

	projectNames := make([]string, len(projectList.Data))

	for idx, project := range projectList.Data {
		projectNames[idx] = project.Name
	}
	sort.Strings(projectNames)
	return projectNames, nil
}

// CreateProjectAndNamespace is a helper to create a project (norman) and a namespace in the project
func CreateProjectAndNamespace(client *rancher.Client, clusterID string) (*management.Project, *corev1.Namespace, error) {
	createdProject, err := client.Management.Project.Create(NewProjectConfig(clusterID))
	if err != nil {
		return nil, nil, err
	}

	namespaceName := namegen.AppendRandomString("testns")
	projectName := strings.Split(createdProject.ID, ":")[1]

	createdNamespace, err := namespaces.CreateNamespace(client, clusterID, projectName, namespaceName, "{}", map[string]string{}, map[string]string{})
	if err != nil {
		return nil, nil, err
	}

	return createdProject, createdNamespace, nil
}

// WaitForProjectFinalizerToUpdate is a helper to wait for project finalizer to update to match the expected finalizer count
func WaitForProjectFinalizerToUpdate(client *rancher.Client, projectName string, projectNamespace string, finalizerCount int) error {
	err := kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.TenSecondTimeout, func() (done bool, pollErr error) {
		project, pollErr := projectsapi.ListProjects(client, projectNamespace, metav1.ListOptions{
			FieldSelector: "metadata.name=" + projectName,
		})
		if pollErr != nil {
			return false, pollErr
		}

		if len(project.Items[0].Finalizers) == finalizerCount {
			return true, nil
		}
		return false, pollErr
	})

	if err != nil {
		return err
	}

	return nil
}

// WaitForProjectIDUpdate is a helper that waits for the project-id annotation and label to be updated in a specified namespace
func WaitForProjectIDUpdate(client *rancher.Client, clusterID, projectName, namespaceName string) error {
	expectedAnnotations := map[string]string{
		projectsapi.ProjectIDAnnotation: clusterID + ":" + projectName,
	}

	expectedLabels := map[string]string{
		projectsapi.ProjectIDAnnotation: projectName,
	}

	err := kwait.Poll(defaults.FiveHundredMillisecondTimeout, defaults.OneMinuteTimeout, func() (done bool, pollErr error) {
		namespace, pollErr := namespaces.GetNamespaceByName(client, clusterID, namespaceName)
		if pollErr != nil {
			return false, pollErr
		}

		for key, expectedValue := range expectedAnnotations {
			if actualValue, ok := namespace.Annotations[key]; !ok || actualValue != expectedValue {
				return false, nil
			}
		}

		for key, expectedValue := range expectedLabels {
			if actualValue, ok := namespace.Labels[key]; !ok || actualValue != expectedValue {
				return false, nil
			}
		}

		return true, nil
	})

	if err != nil {
		return err
	}

	return nil
}

// UpdateProjectNamespaceFinalizer is a helper to update the finalizer in a project
func UpdateProjectNamespaceFinalizer(client *rancher.Client, existingProject *v3.Project, finalizer []string) (*v3.Project, error) {
	updatedProject := existingProject.DeepCopy()
	updatedProject.ObjectMeta.Finalizers = finalizer

	updatedProject, err := projectsapi.UpdateProject(client, existingProject, updatedProject)
	if err != nil {
		return nil, err
	}

	return updatedProject, nil
}
