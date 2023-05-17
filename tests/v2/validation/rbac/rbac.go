package rbac

import (
	"sort"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
)

const (
	roleOwner                = "cluster-owner"
	roleMember               = "cluster-member"
	roleProjectOwner         = "project-owner"
	roleProjectMember        = "project-member"
	roleManageProjectMember  = "projectroletemplatebindings-manage"
	restrictedAdmin          = "restricted-admin"
	standardUser             = "user"
	kubeConfigTokenSettingID = "kubeconfig-default-token-ttl-minutes"
)

type ClusterConfig struct {
	nodeRoles            []string
	externalNodeProvider provisioning.ExternalNodeProvider
	kubernetesVersion    string
	cni                  string
}

func createUser(client *rancher.Client, role string) (*management.User, error) {
	enabled := true
	var username = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: username,
		Password: testpassword,
		Name:     username,
		Enabled:  &enabled,
	}
	newUser, err := users.CreateUserWithRole(client, user, role)
	if err != nil {
		return newUser, err
	}
	newUser.Password = user.Password
	return newUser, nil
}

func listProjects(client *rancher.Client, clusterID string) ([]string, error) {
	projectList, err := projects.GetProjectList(client, clusterID)
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

func getNamespaces(steveclient *v1.Client) ([]string, error) {

	namespaceList, err := steveclient.SteveType(namespaces.NamespaceSteveType).List(nil)
	if err != nil {
		return nil, err
	}

	namespace := make([]string, len(namespaceList.Data))
	for idx, ns := range namespaceList.Data {
		namespace[idx] = ns.GetName()
	}
	sort.Strings(namespace)
	return namespace, nil
}

func deleteNamespace(namespaceID *v1.SteveAPIObject, steveclient *v1.Client) error {
	deletens := steveclient.SteveType(namespaces.NamespaceSteveType).Delete(namespaceID)
	return deletens
}

func createProject(client *rancher.Client, clusterID string) (*management.Project, error) {
	projectName := namegen.AppendRandomString("testproject-")
	projectConfig := &management.Project{
		ClusterID: clusterID,
		Name:      projectName,
	}

	createProject, err := client.Management.Project.Create(projectConfig)
	if err != nil {
		return nil, err
	}
	return createProject, nil
}

func convertSetting(globalSetting *v1.SteveAPIObject) (*v3.Setting, error) {
	updateSetting := &v3.Setting{}
	err := v1.ConvertToK8sType(globalSetting.JSONResp, updateSetting)
	if err != nil {
		return nil, err
	}
	return updateSetting, nil
}

func listGlobalSettings(steveclient *v1.Client) ([]string, error) {
	globalSettings, err := steveclient.SteveType("management.cattle.io.setting").List(nil)
	if err != nil {
		return nil, err
	}

	settingsNameList := make([]string, len(globalSettings.Data))
	for idx, setting := range globalSettings.Data {
		settingsNameList[idx] = setting.Name
	}
	sort.Strings(settingsNameList)
	return settingsNameList, nil
}

func editGlobalSettings(steveclient *v1.Client, globalSetting *v1.SteveAPIObject, value string) (*v1.SteveAPIObject, error) {
	updateSetting, err := convertSetting(globalSetting)
	if err != nil {
		return nil, err
	}

	updateSetting.Value = value
	updateGlobalSetting, err := steveclient.SteveType("management.cattle.io.setting").Update(globalSetting, updateSetting)
	if err != nil {
		return nil, err
	}
	return updateGlobalSetting, nil
}

func getClusterConfig() *ClusterConfig {
	nodeAndRoles := []string{
		"--etcd",
		"--controlplane",
		"--worker",
	}
	userConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, userConfig)

	kubernetesVersion := userConfig.RKE1KubernetesVersions[0]
	cni := userConfig.CNIs[0]
	nodeProviders := userConfig.NodeProviders[0]

	externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)

	clusterConfig := ClusterConfig{nodeRoles: nodeAndRoles, externalNodeProvider: externalNodeProvider,
		kubernetesVersion: kubernetesVersion, cni: cni}

	return &clusterConfig
}
