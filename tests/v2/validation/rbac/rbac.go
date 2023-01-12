package rbac

import (
	"sort"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const roleOwner = "cluster-owner"
const roleMember = "cluster-member"
const roleProjectOwner = "project-owner"
const roleProjectMember = "project-member"

func createUser(client *rancher.Client) (*management.User, error) {
	enabled := true
	var username = namegen.AppendRandomString("testuser-")
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

func getNamespaces(steveclient *v1.Client) (namespace []string, err error) {

	namespaceList, err := steveclient.SteveType(namespaces.NamespaceSteveType).List(nil)
	if err != nil {
		return namespace, err
	}

	namespace = make([]string, len(namespaceList.Data))
	for idx, ns := range namespaceList.Data {
		namespace[idx] = ns.GetName()
	}
	sort.Strings(namespace)
	return namespace, err
}

func deleteNamespace(namespaceID *v1.SteveAPIObject, steveclient *v1.Client) error {
	deletens := steveclient.SteveType(namespaces.NamespaceSteveType).Delete(namespaceID)
	return deletens
}

func createProject(client *rancher.Client, clusterID string) (createProject *management.Project, err error) {
	projectName := namegen.AppendRandomString("testproject-")
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      projectName,
	}

	createProject, err = client.Management.Project.Create(projectConfig)
	return createProject, err

}
