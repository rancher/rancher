package rbac

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/projects"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/extensions/workloads"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	appv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const roleOwner = "cluster-owner"
const roleMember = "cluster-member"
const roleProjectOwner = "project-owner"
const roleProjectMember = "project-member"
const roleProjectReadOnly = "read-only"
const restrictedAdmin = "restricted-admin"
const pssRestrictedPolicy = "restricted"
const pssBaselinePolicy = "baseline"
const pssPrivilegedPolicy = "privileged"
const psaWarn = "pod-security.kubernetes.io/warn"
const psaAudit = "pod-security.kubernetes.io/audit"
const psaEnforce = "pod-security.kubernetes.io/enforce"

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

func getPSALabels(response *v1.SteveAPIObject, actualLabels map[string]string) map[string]string {
	expectedLabels := map[string]string{}

	for label := range response.Labels {
		if _, found := actualLabels[label]; found {
			expectedLabels[label] = actualLabels[label]
		}
	}
	return expectedLabels

}

func createDeployment(steveclient *v1.Client, client *rancher.Client, clusterID string, containerName string, image string, namespaceName string) (*v1.SteveAPIObject, error) {

	deploymentName := namegen.AppendRandomString("")
	containerTemplate := workloads.NewContainer(containerName, image, coreV1.PullAlways, []coreV1.VolumeMount{}, []coreV1.EnvFromSource{})
	matchLabels := map[string]string{}
	matchLabels["workload.user.cattle.io/workloadselector"] = fmt.Sprintf("apps.deployment-%v-%v", namespaceName, deploymentName)

	podTemplate := workloads.NewPodTemplate([]coreV1.Container{containerTemplate}, []coreV1.Volume{}, []coreV1.LocalObjectReference{}, matchLabels)
	deployment := workloads.NewDeploymentTemplate(deploymentName, namespaceName, podTemplate, matchLabels)

	deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).Create(deployment)
	if err != nil {
		return nil, err
	}
	err = kwait.Poll(5*time.Second, 5*time.Minute, func() (done bool, err error) {
		steveclient, err := client.Steve.ProxyDownstream(clusterID)
		if err != nil {
			return false, nil
		}
		deploymentResp, err := steveclient.SteveType(workloads.DeploymentSteveType).ByID(deployment.Namespace + "/" + deployment.Name)
		if err != nil {
			return false, nil
		}
		deployment := &appv1.Deployment{}
		err = v1.ConvertToK8sType(deploymentResp.JSONResp, deployment)
		if err != nil {
			return false, nil
		}
		status := deployment.Status.Conditions
		for _, statusCondition := range status {
			if strings.Contains(statusCondition.Message, "forbidden") {
				err = errors.New(statusCondition.Message)
				return false, err
			}
		}

		if *deployment.Spec.Replicas == deployment.Status.AvailableReplicas {
			return true, nil
		}

		return false, nil
	})
	return deploymentResp, err

}
