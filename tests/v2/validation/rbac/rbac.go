package rbac

import (
	"sort"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	collection "github.com/rancher/rancher/tests/framework/clients/rancher/generated/provisioning/v1"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	provisioning "github.com/rancher/rancher/tests/v2/validation/provisioning"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const roleOwner = "cluster-owner"
const roleMember = "cluster-member"

//Helper function to create users
func createUser(client *rancher.Client) (*management.User, error) {
	enabled := true
	var username = provisioning.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: username,
		Password: testpassword,
		Name:     username,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	if err != nil {
		return newUser, err
	}

	newUser.Password = user.Password
	return newUser, err
}

//Gets the list of projects and return the names of the projects as a slice
func listProjects(client *rancher.Client, clusterID string) (projectNames []string, err error) {
	projectList, err := projects.GetProjectList(client, clusterID)
	if err != nil {
		return projectNames, err
	}

	projectNames = make([]string, len(projectList.Data))

	for idx, project := range projectList.Data {
		projectNames[idx] = project.Name
	}
	sort.Strings(projectNames)
	return projectNames, err
}

//(client *rancher.Client, clusterID string, listOpts metav1.ListOptions

//Gets the list of namespaces and return the names of the namespaces as a slice
func getNamespaces(client *rancher.Client, clusterID string) (namespace []string, err error) {
	namespaceList, err := namespaces.ListNamespaces(client, clusterID, metav1.ListOptions{})
	if err != nil {
		return namespace, err
	}

	namespace = make([]string, len(namespaceList.Items))
	for idx, ns := range namespaceList.Items {
		namespace[idx] = ns.GetName()
	}
	sort.Strings(namespace)
	return namespace, err
}

func createProject(client *rancher.Client, clusterID string) (createProject *management.Project, err error) {
	projectName := provisioning.AppendRandomString("testproject-")
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      projectName,
	}

	createProject, err = client.Management.Project.Create(projectConfig)
	return createProject, err

}

func listClusters(client *rancher.Client) (clusterList *collection.ClusterCollection, err error) {
	clusterList, err = client.Provisioning.Cluster.List(&types.ListOpts{})
	return clusterList, err

}
