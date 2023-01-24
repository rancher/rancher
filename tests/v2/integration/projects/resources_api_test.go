package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/namespaces"
	"github.com/rancher/rancher/tests/framework/extensions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	password "github.com/rancher/rancher/tests/framework/extensions/users/passwordgenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/wrangler/pkg/randomtoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type LocalProjectResourcesAPISetupTestSuite struct {
	projectResourcesAPISetupTestSuite
}

type DownstreamProjectResourcesAPISetupTestSuite struct {
	projectResourcesAPISetupTestSuite
}

type projectResourcesAPISetupTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	clusterID string
}

func (c *projectResourcesAPISetupTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *LocalProjectResourcesAPISetupTestSuite) SetupSuite() {
	c.projectResourcesAPISetupTestSuite.setupSuite("local")
}

func (c *DownstreamProjectResourcesAPISetupTestSuite) SetupSuite() {
	c.projectResourcesAPISetupTestSuite.setupSuite("")
}

func (c *projectResourcesAPISetupTestSuite) setupSuite(clusterName string) {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)
	c.client = client

	if clusterName == "" {
		clusterName = c.client.RancherConfig.ClusterName
	}
	c.clusterID, err = clusters.GetClusterIDByName(client, clusterName)
	require.NoError(c.T(), err)
}

func (c *projectResourcesAPISetupTestSuite) TestResources() {
	projectName, err := randomtoken.Generate()
	require.NoError(c.T(), err)
	projectConfig := &management.Project{
		ClusterID: c.clusterID,
		Name:      projectName[:8],
	}
	project, err := c.client.Management.Project.Create(projectConfig)
	require.NoError(c.T(), err)
	projectID := strings.Split(project.ID, ":")[1]

	username, err := randomtoken.Generate()
	pw := password.GenerateUserPassword("testpass")
	require.NoError(c.T(), err)
	enabled := true
	user1 := &management.User{
		Username: username,
		Name:     username,
		Password: pw,
		Enabled:  &enabled,
	}
	user1, err = users.CreateUserWithRole(c.client, user1, "user")
	require.NoError(c.T(), err)
	err = users.AddProjectMember(c.client, project, user1, "project-owner")
	require.NoError(c.T(), err)

	username, err = randomtoken.Generate()
	pw = password.GenerateUserPassword("testpass")
	require.NoError(c.T(), err)
	user2 := &management.User{
		Username: username,
		Name:     username,
		Password: pw,
		Enabled:  &enabled,
	}
	user2, err = users.CreateUserWithRole(c.client, user2, "user")
	require.NoError(c.T(), err)
	err = users.AddProjectMember(c.client, project, user2, "project-member")
	require.NoError(c.T(), err)

	username, err = randomtoken.Generate()
	pw = password.GenerateUserPassword("testpass")
	require.NoError(c.T(), err)
	user3 := &management.User{
		Username: username,
		Name:     username,
		Password: pw,
		Enabled:  &enabled,
	}
	user3, err = users.CreateUserWithRole(c.client, user3, "user")
	require.NoError(c.T(), err)
	if c.clusterID != "local" {
		cluster, err := c.client.Management.Cluster.ByID(c.clusterID)
		require.NoError(c.T(), err)
		err = users.AddClusterRoleToUser(c.client, cluster, user3, "cluster-owner")
		require.NoError(c.T(), err)
	}

	var roleBindings *rbacv1.RoleBindingList
	err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
		roleBindings, err = rbac.ListRoleBindings(c.client, project.ClusterID, projectID, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		if len(roleBindings.Items) >= 4 {
			return true, nil
		}
		return false, nil
	})
	require.NoError(c.T(), err)

	if c.clusterID != "local" {
		err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
			clusterRoleBindings, err := rbac.ListClusterRoleBindings(c.client, project.ClusterID, metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			for _, crb := range clusterRoleBindings.Items {
				if len(crb.Subjects) == 1 && crb.Subjects[0].Name == user3.ID && crb.RoleRef.Name == "cluster-owner-projectresources" {
					return true, nil
				}
			}
			return false, nil
		})
		require.NoError(c.T(), err)
	}

	cr, err := rbac.GetClusterRoleByName(c.client, project.ClusterID, "project-owner-projectresources")
	require.NoError(c.T(), err)
	require.NotNil(c.T(), cr)

	namespace, err := namespaces.GetNamespaceByName(c.client, project.ClusterID, projectID)
	require.NoError(c.T(), err)
	require.NotNil(c.T(), namespace)

	rolesUsers := map[string]string{
		"admin-projectresources":          user1.ID,
		"project-owner-projectresources":  user1.ID,
		"edit-projectresources":           user2.ID,
		"project-member-projectresources": user2.ID,
	}
	for _, rb := range roleBindings.Items {
		user, ok := rolesUsers[rb.RoleRef.Name]
		if !ok {
			continue
		}
		if len(rb.Subjects) != 1 {
			continue
		}
		if user == rb.Subjects[0].Name {
			delete(rolesUsers, rb.RoleRef.Name)
		}
	}
	assert.Len(c.T(), rolesUsers, 0, "did not find expected roles in rolebindings")

	err = users.RemoveProjectMember(c.client, user1)
	require.NoError(c.T(), err)

	err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
		roleBindings, err = rbac.ListRoleBindings(c.client, project.ClusterID, projectID, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		for _, rb := range roleBindings.Items {
			if len(rb.Subjects) == 1 && rb.Subjects[0].Name == user1.ID {
				return false, nil
			}
		}
		return true, nil
	})
	require.NoError(c.T(), err)

	if c.clusterID != "local" {
		err = users.RemoveClusterRoleFromUser(c.client, user3)
		require.NoError(c.T(), err)
		err = wait.Poll(time.Second, time.Minute, func() (done bool, err error) {
			clusterRoleBindings, err := rbac.ListClusterRoleBindings(c.client, project.ClusterID, metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			for _, crb := range clusterRoleBindings.Items {
				if len(crb.Subjects) == 1 && crb.Subjects[0].Name == user3.ID {
					return false, nil
				}
			}
			return true, nil
		})
		require.NoError(c.T(), err)
	}

	err = c.client.Management.Project.Delete(project)
	require.NoError(c.T(), err)

	err = wait.Poll(time.Second, 2*time.Minute, func() (done bool, err error) {
		namespace, _ = namespaces.GetNamespaceByName(c.client, project.ClusterID, projectID)
		if namespace == nil {
			return true, nil
		}
		return false, nil
	})

	namespace, err = namespaces.GetNamespaceByName(c.client, project.ClusterID, "cattle-unscoped")
	require.NoError(c.T(), err)
	require.NotNil(c.T(), namespace)

	unscopedRoleBinding, err := rbac.GetRoleBindingByName(c.client, project.ClusterID, "cattle-unscoped", "cattle-project-resources")
	require.NoError(c.T(), err)
	require.NotNil(c.T(), unscopedRoleBinding)
}

func TestLocalProjectResourcesAPISetup(t *testing.T) {
	suite.Run(t, new(LocalProjectResourcesAPISetupTestSuite))
}

func TestDownstreamProjectResourcesAPISetup(t *testing.T) {
	suite.Run(t, new(DownstreamProjectResourcesAPISetupTestSuite))
}
