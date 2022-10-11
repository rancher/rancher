package users

import (
	"strings"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

// CreateUserWithRole is helper function that creates a user with a role or multiple roles
func CreateUserWithRole(rancherClient *rancher.Client, user *management.User, roles ...string) (*management.User, error) {
	createdUser, err := rancherClient.Management.User.Create(user)
	if err != nil {
		return nil, err
	}

	for _, role := range roles {
		roleBinding := &management.GlobalRoleBinding{
			GlobalRoleID: role,
			UserID:       createdUser.ID,
		}

		_, err = rancherClient.Management.GlobalRoleBinding.Create(roleBinding)
		if err != nil {
			return nil, err
		}
	}

	return createdUser, nil
}

// AddProjectMember is a helper function that adds a project role to `user`. It uses the watch.WatchWait ensure BackingNamespaceCreated is true
func AddProjectMember(rancherClient *rancher.Client, project *management.Project, user *management.User, projectRole string) error {
	role := &management.ProjectRoleTemplateBinding{
		ProjectID:       project.ID,
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  projectRole,
	}

	name := strings.Split(project.ID, ":")[1]

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + name,
		TimeoutSeconds: &defaults.WatchTimeoutSeconds,
	}
	watchInterface, err := rancherClient.GetManagementWatchInterface(management.ProjectType, opts)
	if err != nil {
		return err
	}

	checkFunc := func(event watch.Event) (ready bool, err error) {
		projectUnstructured := event.Object.(*unstructured.Unstructured)
		project := &v3.Project{}
		err = scheme.Scheme.Convert(projectUnstructured, project, projectUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if v3.NamespaceBackedResource.IsTrue(project) {
			return true, nil
		}

		return false, nil
	}

	err = wait.WatchWait(watchInterface, checkFunc)
	if err != nil {
		return err
	}

	roleTemplateResp, err := rancherClient.Management.ProjectRoleTemplateBinding.Create(role)

	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		projectRoleTemplate, err := rancherClient.Management.ProjectRoleTemplateBinding.ByID(roleTemplateResp.ID)
		if err != nil {
			return false, err
		}
		if projectRoleTemplate != nil {
			return true, nil
		}

		return false, nil
	})
	return err
}

// RemoveProjectMember is a helper function that removes the project role from `user`
func RemoveProjectMember(rancherClient *rancher.Client, user *management.User) error {
	roles, err := rancherClient.Management.ProjectRoleTemplateBinding.List(&types.ListOpts{})
	if err != nil {
		return err
	}

	var roleToDelete management.ProjectRoleTemplateBinding

	for _, role := range roles.Data {
		if role.UserID == user.ID {
			roleToDelete = role
			break
		}
	}
	return rancherClient.Management.ProjectRoleTemplateBinding.Delete(&roleToDelete)
}
