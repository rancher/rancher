//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package secrets

import (
	"testing"

	projectsapi "github.com/rancher/rancher/tests/v2/actions/projects"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
	secret "github.com/rancher/rancher/tests/v2/actions/secrets"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wrangler"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RbacSecretTestSuite struct {
	suite.Suite
	client            *rancher.Client
	session           *session.Session
	cluster           *management.Cluster
	downstreamContext *wrangler.Context
}

func (rbs *RbacSecretTestSuite) TearDownSuite() {
	rbs.session.Cleanup()
}

func (rbs *RbacSecretTestSuite) SetupSuite() {
	rbs.session = session.NewSession()

	client, err := rancher.NewClient("", rbs.session)
	assert.NoError(rbs.T(), err)
	rbs.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rb")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rbs.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rbs.client, clusterName)
	require.NoError(rbs.T(), err, "Error getting cluster ID")
	rbs.cluster, err = rbs.client.Management.Cluster.ByID(clusterID)
	assert.NoError(rbs.T(), err)
}

func (rbs *RbacSecretTestSuite) TestCreateSecretAsEnvVar() {
	subSession := rbs.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rbs.Run("Validate secret creation for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projectsapi.CreateProjectAndNamespace(rbs.client, rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbs.client, tt.member, tt.role.String(), rbs.cluster, adminProject)
			assert.NoError(rbs.T(), err)
			rbs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", tt.role.String(), adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secret.CreateSecret(standardUserClient, rbs.cluster.ID, namespace.Name, secretData)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbs.T(), err, "failed to create secret")
				log.Infof("As a %v, create a deployment using the secret as an environment variable.", tt.role.String())
				_, err = deployment.CreateDeployment(standardUserClient, rbs.cluster.ID, namespace.Name, 1, createdSecret.Name, "", true, false)
				assert.NoError(rbs.T(), err, "failed to create deployment with secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbs.T(), err)
				assert.True(rbs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbs *RbacSecretTestSuite) TestCreateSecretAsVolume() {
	subSession := rbs.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rbs.Run("Validate secret creation for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projectsapi.CreateProjectAndNamespace(rbs.client, rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbs.client, tt.member, tt.role.String(), rbs.cluster, adminProject)
			assert.NoError(rbs.T(), err)
			rbs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", tt.role.String(), adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secret.CreateSecret(standardUserClient, rbs.cluster.ID, namespace.Name, secretData)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbs.T(), err, "failed to create secret")
				log.Infof("As a %v, create a deployment using the secret as an environment variable.", tt.role.String())
				_, err = deployment.CreateDeployment(standardUserClient, rbs.cluster.ID, namespace.Name, 1, createdSecret.Name, "", false, true)
				assert.NoError(rbs.T(), err, "failed to create deployment with secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbs.T(), err)
				assert.True(rbs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbs *RbacSecretTestSuite) TestListSecret() {
	subSession := rbs.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rbs.Run("Validate listing secret for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projectsapi.CreateProjectAndNamespace(rbs.client, rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbs.client, tt.member, tt.role.String(), rbs.cluster, adminProject)
			assert.NoError(rbs.T(), err)
			rbs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", rbac.Admin, adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secret.CreateSecret(rbs.client, rbs.cluster.ID, namespace.Name, secretData)
			assert.NoError(rbs.T(), err, "failed to create secret")

			log.Infof("As a %v, list the secret.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbs.cluster.ID)
			assert.NoError(rbs.T(), err)
			secretList, err := standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbs.T(), err, "failed to list secret")
				assert.Equal(rbs.T(), len(secretList.Items), 1)
				assert.Equal(rbs.T(), secretList.Items[0].Name, createdSecret.Name)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbs.T(), err)
				assert.True(rbs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbs *RbacSecretTestSuite) TestUpdateSecret() {
	subSession := rbs.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rbs.Run("Validate updating secret as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projectsapi.CreateProjectAndNamespace(rbs.client, rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbs.client, tt.member, tt.role.String(), rbs.cluster, adminProject)
			assert.NoError(rbs.T(), err)
			rbs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", rbac.Admin, adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secret.CreateSecret(rbs.client, rbs.cluster.ID, namespace.Name, secretData)
			assert.NoError(rbs.T(), err, "failed to create secret")

			log.Infof("As a %v, update the secret.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			newData := map[string][]byte{
				"foo": []byte("bar"),
			}
			updatedSecretObj := secret.UpdateSecretData(createdSecret, newData)
			updatedSecret, err := standardUserContext.Core.Secret().Update(updatedSecretObj)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbs.T(), err, "failed to update secret")
				assert.NotNil(rbs.T(), updatedSecret)
				assert.Contains(rbs.T(), updatedSecret.Data, "foo")
				assert.Equal(rbs.T(), updatedSecret.Data["foo"], []byte("bar"))

				log.Infof("As a %v, create a deployment using the updated secret.", tt.role.String())
				_, err = deployment.CreateDeployment(standardUserClient, rbs.cluster.ID, namespace.Name, 1, updatedSecret.Name, "", true, false)
				assert.NoError(rbs.T(), err, "failed to create deployment with secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbs.T(), err)
				assert.True(rbs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbs *RbacSecretTestSuite) TestDeleteSecret() {
	subSession := rbs.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rbs.Run("Validate deleting secret as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projectsapi.CreateProjectAndNamespace(rbs.client, rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbs.client, tt.member, tt.role.String(), rbs.cluster, adminProject)
			assert.NoError(rbs.T(), err)
			rbs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", rbac.Admin, adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secret.CreateSecret(rbs.client, rbs.cluster.ID, namespace.Name, secretData)
			assert.NoError(rbs.T(), err, "failed to create secret")

			log.Infof("As a %v, delete the secret.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbs.cluster.ID)
			assert.NoError(rbs.T(), err)

			err = standardUserContext.Core.Secret().Delete(namespace.Name, createdSecret.Name, &metav1.DeleteOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbs.T(), err, "failed to delete secret")
				secretList, err := rbs.client.WranglerContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
				assert.NoError(rbs.T(), err)
				assert.Equal(rbs.T(), len(secretList.Items), 0)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbs.T(), err)
				assert.True(rbs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbs *RbacSecretTestSuite) TestCrudSecretAsClusterMember() {
	subSession := rbs.session.NewSession()
	defer subSession.Cleanup()

	role := rbac.ClusterMember.String()
	log.Infof("Create a standard user.")
	user, standardUserClient, err := rbac.SetupUser(rbs.client, rbac.StandardUser.String())
	assert.NoError(rbs.T(), err)

	log.Infof("Add the user to the downstream cluster with role %s", role)
	err = users.AddClusterRoleToUser(rbs.client, rbs.cluster, user, role, nil)
	assert.NoError(rbs.T(), err)

	log.Infof("As a %v, create a project and a namespace in the project.", role)
	project, namespace, err := projectsapi.CreateProjectAndNamespace(standardUserClient, rbs.cluster.ID)
	assert.NoError(rbs.T(), err)

	log.Infof("As a %v, create a secret in the project %v", role, project.Name)
	secretData := map[string][]byte{
		"hello": []byte("world"),
	}
	createdSecret, err := secret.CreateSecret(standardUserClient, rbs.cluster.ID, namespace.Name, secretData)
	require.NoError(rbs.T(), err, "failed to create secret")

	log.Infof("As a %v, create a deployment using the secret as an environment variable.", role)
	_, err = deployment.CreateDeployment(standardUserClient, rbs.cluster.ID, namespace.Name, 1, createdSecret.Name, "", true, false)
	require.NoError(rbs.T(), err, "failed to create deployment with secret")

	log.Infof("As a %v, list the secret.", role)
	standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbs.cluster.ID)
	assert.NoError(rbs.T(), err)
	secretList, err := standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
	require.NoError(rbs.T(), err, "failed to list secret")
	require.Equal(rbs.T(), len(secretList.Items), 1)
	require.Equal(rbs.T(), secretList.Items[0].Name, createdSecret.Name)

	log.Infof("As a %v, update the secret.", role)
	newData := map[string][]byte{
		"foo": []byte("bar"),
	}
	updatedSecretObj := secret.UpdateSecretData(createdSecret, newData)
	updatedSecret, err := standardUserContext.Core.Secret().Update(updatedSecretObj)
	require.NoError(rbs.T(), err, "failed to list secret")
	assert.NotNil(rbs.T(), updatedSecret)
	assert.Contains(rbs.T(), updatedSecret.Data, "foo")
	assert.Equal(rbs.T(), updatedSecret.Data["foo"], []byte("bar"))

	log.Infof("As a %v, create a deployment using the secret as an environment variable.", role)
	_, err = deployment.CreateDeployment(standardUserClient, rbs.cluster.ID, namespace.Name, 1, updatedSecret.Name, "", true, false)
	require.NoError(rbs.T(), err, "failed to create deployment with secret")

	log.Infof("As a %v, delete the secret.", role)
	err = standardUserContext.Core.Secret().Delete(namespace.Name, updatedSecret.Name, &metav1.DeleteOptions{})
	require.NoError(rbs.T(), err, "failed to delete secret")
	secretList, err = standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
	assert.NoError(rbs.T(), err)
	assert.Equal(rbs.T(), len(secretList.Items), 0)
}

func TestRbacSecretTestSuite(t *testing.T) {
	suite.Run(t, new(RbacSecretTestSuite))
}
