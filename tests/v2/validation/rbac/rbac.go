package rbac

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
)

const (
	defaultRandStringLength = 5
)


//Helper to create random names for namespaces & projects 
func AppendRandomString(baseName string) string {
	clusterName := "auto-" + baseName + "-" + namegenerator.RandStringLower(defaultRandStringLength)
	return clusterName
}

//Helper function to create users 
func CreateUser(client *rancher.Client)(*management.User,error) {
	enabled := true
	var username = AppendRandomString("testuser-")
	user := &management.User{
		Username: username,
		Password: "rancherrancher123",
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
func GetListProjects(client *rancher.Client, clusterID string) (projectNames []string, err error) {
	projectList, err := projects.GetProjectList(client, clusterID)
	projectNames = make([]string, len(projectList.Data))

	if err != nil {
		return projectNames, err
	}

	for idx, project := range projectList.Data {
		projectNames[idx] = project.Name
	}
	return projectNames, err
}

//Gets the list of namespaces and return the names of the namespaces as a slice
func GetListNamespaces(client *rancher.Client, clusterID string) (namespace []string, err error) {
	namespaceList, err := namespaces.ListNamespace(client, clusterID)
	if err != nil {
		return namespace, err
	}

	namespace = make([]string, len(namespaceList.Items))
	for idx, ns := range namespaceList.Items {
		namespace[idx] = ns.GetName()
	}
	return namespace, err
}

func CreateProject(client *rancher.Client, clusterID string) (project *management.Project, err error){
	projectName := AppendRandomString("testproject-CO-")
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      projectName,
	}
	createProject, err := client.Management.Project.Create(projectConfig)
	return createProject, err	

}