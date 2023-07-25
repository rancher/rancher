package users

import (
	"fmt"
	"strings"
	"time"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/pkg/api/scheme"
	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/ref"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/rbac"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	namegen "github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	rtbOwnerLabel = "authz.cluster.cattle.io/rtb-owner-updated"
)

var timeout = int64(60 * 3)

// UserConfig sets and returns username and password of the user
func UserConfig() (user *management.User) {
	enabled := true
	var username = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user = &management.User{
		Username: username,
		Password: testpassword,
		Name:     username,
		Enabled:  &enabled,
	}

	return
}

// CreateUserWithRole is helper function that creates a user with a role or multiple roles
func CreateUserWithRole(rancherClient *rancher.Client, user *management.User, roles ...string) (*management.User, error) {
	createdUser, err := rancherClient.Management.User.Create(user)
	if err != nil {
		return nil, err
	} else {
		createdUser.Password = user.Password
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

	adminClient, err := rancher.NewClient(rancherClient.RancherConfig.AdminToken, rancherClient.Session)
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + name,
		TimeoutSeconds: &timeout,
	}
	watchInterface, err := adminClient.GetManagementWatchInterface(management.ProjectType, opts)
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
	if err != nil {
		return err
	}

	var prtb *management.ProjectRoleTemplateBinding
	err = kwait.Poll(500*time.Millisecond, 2*time.Minute, func() (done bool, err error) {
		prtb, err = rancherClient.Management.ProjectRoleTemplateBinding.ByID(roleTemplateResp.ID)
		if err != nil {
			return false, err
		}
		if prtb != nil && prtb.UserID == user.ID && prtb.ProjectID == project.ID {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}
	return waitForPRTBRollout(rancherClient, prtb, createOp)
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

	err = rancherClient.Management.ProjectRoleTemplateBinding.Delete(&roleToDelete)
	if err != nil {
		return err
	}
	return waitForPRTBRollout(rancherClient, &roleToDelete, deleteOp)
}

// AddClusterRoleToUser is a helper function that adds a cluster role to `user`.
func AddClusterRoleToUser(rancherClient *rancher.Client, cluster *management.Cluster, user *management.User, clusterRole string) error {
	role := &management.ClusterRoleTemplateBinding{
		ClusterID:       cluster.Resource.ID,
		UserPrincipalID: user.PrincipalIDs[0],
		RoleTemplateID:  clusterRole,
	}

	adminClient, err := rancher.NewClient(rancherClient.RancherConfig.AdminToken, rancherClient.Session)
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{
		FieldSelector:  "metadata.name=" + cluster.ID,
		TimeoutSeconds: &timeout,
	}
	watchInterface, err := adminClient.GetManagementWatchInterface(management.ClusterType, opts)
	if err != nil {
		return err
	}

	checkFunc := func(event watch.Event) (ready bool, err error) {
		clusterUnstructured := event.Object.(*unstructured.Unstructured)
		cluster := &v3.Cluster{}

		err = scheme.Scheme.Convert(clusterUnstructured, cluster, clusterUnstructured.GroupVersionKind())
		if err != nil {
			return false, err
		}
		if cluster.Annotations == nil || cluster.Annotations["field.cattle.io/creatorId"] == "" {
			// no cluster creator, no roles to populate. This will be the case for the "local" cluster.
			return true, nil
		}

		v3.ClusterConditionInitialRolesPopulated.CreateUnknownIfNotExists(cluster)
		if v3.ClusterConditionInitialRolesPopulated.IsUnknown(cluster) || v3.ClusterConditionInitialRolesPopulated.IsTrue(cluster) {
			return true, nil
		}
		return false, nil
	}

	err = wait.WatchWait(watchInterface, checkFunc)
	if err != nil {
		return err
	}

	roleTemplateResp, err := rancherClient.Management.ClusterRoleTemplateBinding.Create(role)
	if err != nil {
		return err
	}

	var crtb *management.ClusterRoleTemplateBinding
	err = kwait.Poll(600*time.Millisecond, 3*time.Minute, func() (done bool, err error) {
		crtb, err = rancherClient.Management.ClusterRoleTemplateBinding.ByID(roleTemplateResp.ID)
		if err != nil {
			return false, err
		}
		if crtb != nil {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}
	return waitForCRTBRollout(rancherClient, crtb, createOp)
}

// RemoveClusterRoleFromUser is a helper function that removes the user from cluster
func RemoveClusterRoleFromUser(rancherClient *rancher.Client, user *management.User) error {
	roles, err := rancherClient.Management.ClusterRoleTemplateBinding.List(&types.ListOpts{})
	if err != nil {
		return err
	}

	var roleToDelete management.ClusterRoleTemplateBinding

	for _, role := range roles.Data {
		if role.UserID == user.ID {
			roleToDelete = role
			break
		}
	}

	if err = rancherClient.Management.ClusterRoleTemplateBinding.Delete(&roleToDelete); err != nil {
		return err
	}
	return waitForCRTBRollout(rancherClient, &roleToDelete, deleteOp)
}

// GetUserIDByName is a helper function that returns the user ID by name
func GetUserIDByName(client *rancher.Client, username string) (string, error) {
	userList, err := client.Management.User.List(&types.ListOpts{})
	if err != nil {
		return "", err
	}

	if err != nil {
		return "", err
	}

	for _, user := range userList.Data {
		if user.Username == username {
			return user.ID, nil
		}
	}

	return "", nil
}

type operationType int

const (
	createOp operationType = iota
	deleteOp
)

func waitForCRTBRollout(client *rancher.Client, crtb *management.ClusterRoleTemplateBinding, opType operationType) error {
	crtbNamespace, crtbName := ref.Parse(crtb.ID)
	req, err := labels.NewRequirement(rtbOwnerLabel, selection.In, []string{fmt.Sprintf("%s_%s", crtbNamespace, crtbName)})
	if err != nil {
		return fmt.Errorf("unable to form label requirement for %s/%s: %w", crtbNamespace, crtbName, err)
	}
	selector := labels.NewSelector().Add(*req)
	return waitForRTBRollout(client, crtbNamespace, crtbName, selector, crtb.ClusterID, opType)
}

func waitForPRTBRollout(client *rancher.Client, prtb *management.ProjectRoleTemplateBinding, opType operationType) error {
	clusterID, _ := ref.Parse(prtb.ProjectID)
	prtbNamespace, prtbName := ref.Parse(prtb.ID)
	req, err := labels.NewRequirement(fmt.Sprintf("%s_%s", prtbNamespace, prtbName), selection.Exists, nil)
	if err != nil {
		return fmt.Errorf("unable to form label requirement for %s/%s: %w", prtbNamespace, prtbName, err)
	}
	selector := labels.NewSelector().Add(*req)
	return waitForRTBRollout(client, prtbNamespace, prtbName, selector, clusterID, opType)
}

func waitForRTBRollout(client *rancher.Client, rtbNamespace string, rtbName string, selector labels.Selector, clusterID string, opType operationType) error {
	// we expect rollout to happen within 5 seconds total
	backoff := kwait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1,
		Jitter:   0,
		Steps:    11,
	}
	err := kwait.ExponentialBackoff(backoff, func() (done bool, err error) {
		downstreamCRBs, err := rbac.ListClusterRoleBindings(client, clusterID, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return false, err
		}
		switch opType {
		case createOp:
			return len(downstreamCRBs.Items) > 0, nil
		case deleteOp:
			return len(downstreamCRBs.Items) == 0, nil
		default:
			// unknown operation type, don't poll infinitely
			return true, nil
		}
	})
	if err != nil {
		return fmt.Errorf("unable to determine the status of backing rbac for %s/%s in alloted duration: %w", rtbNamespace, rtbName, err)
	}
	return nil
}
