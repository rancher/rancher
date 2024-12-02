//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package secrets

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/secrets"
	"github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RbacOpaqueSecretTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (rbos *RbacOpaqueSecretTestSuite) TearDownSuite() {
	rbos.session.Cleanup()
}

func (rbos *RbacOpaqueSecretTestSuite) SetupSuite() {
	rbos.session = session.NewSession()

	client, err := rancher.NewClient("", rbos.session)
	assert.NoError(rbos.T(), err)
	rbos.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rbos")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rbos.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rbos.client, clusterName)
	require.NoError(rbos.T(), err, "Error getting cluster ID")
	rbos.cluster, err = rbos.client.Management.Cluster.ByID(clusterID)
	assert.NoError(rbos.T(), err)
}

func (rbos *RbacOpaqueSecretTestSuite) TestCreateSecretAsEnvVar() {
	subSession := rbos.session.NewSession()
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
		rbos.Run("Validate secret creation for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbos.client, rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbos.client, tt.member, tt.role.String(), rbos.cluster, adminProject)
			assert.NoError(rbos.T(), err)
			rbos.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", tt.role.String(), adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secrets.CreateSecret(standardUserClient, rbos.cluster.ID, namespace.Name, secretData, corev1.SecretTypeOpaque)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbos.T(), err, "failed to create secret")
				log.Infof("As a %v, create a deployment using the secret as an environment variable.", tt.role.String())
				_, err = deployment.CreateDeployment(standardUserClient, rbos.cluster.ID, namespace.Name, 1, createdSecret.Name, "", true, false, false, true)
				assert.NoError(rbos.T(), err, "failed to create deployment with secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbos.T(), err)
				assert.True(rbos.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbos *RbacOpaqueSecretTestSuite) TestCreateSecretAsVolume() {
	subSession := rbos.session.NewSession()
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
		rbos.Run("Validate secret creation for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbos.client, rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbos.client, tt.member, tt.role.String(), rbos.cluster, adminProject)
			assert.NoError(rbos.T(), err)
			rbos.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", tt.role.String(), adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secrets.CreateSecret(standardUserClient, rbos.cluster.ID, namespace.Name, secretData, corev1.SecretTypeOpaque)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbos.T(), err, "failed to create secret")
				log.Infof("As a %v, create a deployment using the secret as an environment variable.", tt.role.String())
				_, err = deployment.CreateDeployment(standardUserClient, rbos.cluster.ID, namespace.Name, 1, createdSecret.Name, "", false, true, false, true)
				assert.NoError(rbos.T(), err, "failed to create deployment with secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbos.T(), err)
				assert.True(rbos.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbos *RbacOpaqueSecretTestSuite) TestListSecret() {
	subSession := rbos.session.NewSession()
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
		rbos.Run("Validate listing secret for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbos.client, rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbos.client, tt.member, tt.role.String(), rbos.cluster, adminProject)
			assert.NoError(rbos.T(), err)
			rbos.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", rbac.Admin, adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secrets.CreateSecret(rbos.client, rbos.cluster.ID, namespace.Name, secretData, corev1.SecretTypeOpaque)
			assert.NoError(rbos.T(), err, "failed to create secret")

			log.Infof("As a %v, list the secrets.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbos.cluster.ID)
			assert.NoError(rbos.T(), err)
			secretList, err := standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbos.T(), err, "failed to list secret")
				assert.Equal(rbos.T(), len(secretList.Items), 1)
				assert.Equal(rbos.T(), secretList.Items[0].Name, createdSecret.Name)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbos.T(), err)
				assert.True(rbos.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbos *RbacOpaqueSecretTestSuite) TestUpdateSecret() {
	subSession := rbos.session.NewSession()
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
		rbos.Run("Validate updating secret as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbos.client, rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbos.client, tt.member, tt.role.String(), rbos.cluster, adminProject)
			assert.NoError(rbos.T(), err)
			rbos.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", rbac.Admin, adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secrets.CreateSecret(rbos.client, rbos.cluster.ID, namespace.Name, secretData, corev1.SecretTypeOpaque)
			assert.NoError(rbos.T(), err, "failed to create secret")

			log.Infof("As a %v, update the secrets.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			newData := map[string][]byte{
				"foo": []byte("bar"),
			}
			updatedSecretObj := secrets.UpdateSecretData(createdSecret, newData)
			updatedSecret, err := standardUserContext.Core.Secret().Update(updatedSecretObj)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbos.T(), err, "failed to update secret")
				assert.NotNil(rbos.T(), updatedSecret)
				assert.Contains(rbos.T(), updatedSecret.Data, "foo")
				assert.Equal(rbos.T(), updatedSecret.Data["foo"], []byte("bar"))

				log.Infof("As a %v, create a deployment using the updated secrets.", tt.role.String())
				_, err = deployment.CreateDeployment(standardUserClient, rbos.cluster.ID, namespace.Name, 1, updatedSecret.Name, "", true, false, false, true)
				assert.NoError(rbos.T(), err, "failed to create deployment with secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbos.T(), err)
				assert.True(rbos.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbos *RbacOpaqueSecretTestSuite) TestDeleteSecret() {
	subSession := rbos.session.NewSession()
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
		rbos.Run("Validate deleting secret as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbos.client, rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbos.client, tt.member, tt.role.String(), rbos.cluster, adminProject)
			assert.NoError(rbos.T(), err)
			rbos.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a secret in the project %v", rbac.Admin, adminProject.Name)
			secretData := map[string][]byte{
				"hello": []byte("world"),
			}
			createdSecret, err := secrets.CreateSecret(rbos.client, rbos.cluster.ID, namespace.Name, secretData, corev1.SecretTypeOpaque)
			assert.NoError(rbos.T(), err, "failed to create secret")

			log.Infof("As a %v, delete the secrets.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbos.cluster.ID)
			assert.NoError(rbos.T(), err)

			err = standardUserContext.Core.Secret().Delete(namespace.Name, createdSecret.Name, &metav1.DeleteOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbos.T(), err, "failed to delete secret")
				secretList, err := rbos.client.WranglerContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
				assert.NoError(rbos.T(), err)
				assert.Equal(rbos.T(), len(secretList.Items), 0)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbos.T(), err)
				assert.True(rbos.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbos *RbacOpaqueSecretTestSuite) TestCrudSecretAsClusterMember() {
	subSession := rbos.session.NewSession()
	defer subSession.Cleanup()

	role := rbac.ClusterMember.String()
	log.Infof("Create a standard user.")
	user, standardUserClient, err := rbac.SetupUser(rbos.client, rbac.StandardUser.String())
	assert.NoError(rbos.T(), err)

	log.Infof("Add the user to the downstream cluster with role %s", role)
	err = users.AddClusterRoleToUser(rbos.client, rbos.cluster, user, role, nil)
	assert.NoError(rbos.T(), err)

	log.Infof("As a %v, create a project and a namespace in the project.", role)
	project, namespace, err := projects.CreateProjectAndNamespace(standardUserClient, rbos.cluster.ID)
	assert.NoError(rbos.T(), err)

	log.Infof("As a %v, create a secret in the project %v", role, project.Name)
	secretData := map[string][]byte{
		"hello": []byte("world"),
	}
	createdSecret, err := secrets.CreateSecret(standardUserClient, rbos.cluster.ID, namespace.Name, secretData, corev1.SecretTypeOpaque)
	require.NoError(rbos.T(), err, "failed to create secret")

	log.Infof("As a %v, create a deployment using the secret as an environment variable.", role)
	_, err = deployment.CreateDeployment(standardUserClient, rbos.cluster.ID, namespace.Name, 1, createdSecret.Name, "", true, false, false, true)
	require.NoError(rbos.T(), err, "failed to create deployment with secret")

	log.Infof("As a %v, list the secrets.", role)
	standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbos.cluster.ID)
	assert.NoError(rbos.T(), err)
	secretList, err := standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
	require.NoError(rbos.T(), err, "failed to list secret")
	require.Equal(rbos.T(), len(secretList.Items), 1)
	require.Equal(rbos.T(), secretList.Items[0].Name, createdSecret.Name)

	log.Infof("As a %v, update the secrets.", role)
	newData := map[string][]byte{
		"foo": []byte("bar"),
	}
	updatedSecretObj := secrets.UpdateSecretData(&secretList.Items[0], newData)
	updatedSecret, err := standardUserContext.Core.Secret().Update(updatedSecretObj)
	require.NoError(rbos.T(), err, "failed to update secret")
	assert.NotNil(rbos.T(), updatedSecret)
	assert.Contains(rbos.T(), updatedSecret.Data, "foo")
	assert.Equal(rbos.T(), updatedSecret.Data["foo"], []byte("bar"))

	log.Infof("As a %v, create a deployment using the secret as an environment variable.", role)
	_, err = deployment.CreateDeployment(standardUserClient, rbos.cluster.ID, namespace.Name, 1, updatedSecret.Name, "", true, false, false, true)
	require.NoError(rbos.T(), err, "failed to create deployment with secret")

	log.Infof("As a %v, delete the secrets.", role)
	err = standardUserContext.Core.Secret().Delete(namespace.Name, updatedSecret.Name, &metav1.DeleteOptions{})
	require.NoError(rbos.T(), err, "failed to delete secret")
	secretList, err = standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
	assert.NoError(rbos.T(), err)
	assert.Equal(rbos.T(), len(secretList.Items), 0)
}

func TestRbacOpaqueSecretTestSuite(t *testing.T) {
	suite.Run(t, new(RbacOpaqueSecretTestSuite))
}
