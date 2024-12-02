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
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RbacRegistrySecretTestSuite struct {
	suite.Suite
	client         *rancher.Client
	session        *session.Session
	cluster        *management.Cluster
	registryConfig *secrets.Config
}

func (rbrs *RbacRegistrySecretTestSuite) TearDownSuite() {
	rbrs.session.Cleanup()
}

func (rbrs *RbacRegistrySecretTestSuite) SetupSuite() {
	rbrs.session = session.NewSession()

	client, err := rancher.NewClient("", rbrs.session)
	assert.NoError(rbrs.T(), err)
	rbrs.client = client

	log.Info("Getting cluster name and registry credentials from the config file and append the details in rbrs")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rbrs.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rbrs.client, clusterName)
	require.NoError(rbrs.T(), err, "Error getting cluster ID")
	rbrs.cluster, err = rbrs.client.Management.Cluster.ByID(clusterID)
	assert.NoError(rbrs.T(), err)

	rbrs.registryConfig = new(secrets.Config)
	config.LoadConfig(secrets.ConfigurationFileKey, rbrs.registryConfig)
}

func (rbrs *RbacRegistrySecretTestSuite) TestCreateRegistrySecret() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	dockerConfigJSON, err := secrets.CreateRegistrySecretDockerConfigJSON(rbrs.registryConfig)
	assert.NoError(rbrs.T(), err)
	secretData := map[string][]byte{
		corev1.DockerConfigJsonKey: []byte(dockerConfigJSON),
	}

	for _, tt := range tests {
		subSession := rbrs.session.NewSession()
		defer subSession.Cleanup()

		rbrs.Run("Validate registry secret creation for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the projects.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbrs.client, rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbrs.client, tt.member, tt.role.String(), rbrs.cluster, adminProject)
			assert.NoError(rbrs.T(), err)
			rbrs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a registry secret in the namespace %v", tt.role.String(), namespace.Name)
			createdRegistrySecret, err := secrets.CreateSecret(standardUserClient, rbrs.cluster.ID, namespace.Name, secretData, corev1.SecretTypeDockerConfigJson)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbrs.T(), err, "failed to create a registry secret")
				log.Infof("As a %v, create a deployment using the registry secrets.", tt.role.String())
				_, err := deployment.CreateDeployment(standardUserClient, rbrs.cluster.ID, namespace.Name, 1, createdRegistrySecret.Name, "", false, false, true, true)
				assert.NoError(rbrs.T(), err, "failed to create deployment with registry secret")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbrs.T(), err)
				assert.True(rbrs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbrs *RbacRegistrySecretTestSuite) TestListRegistrySecret() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	dockerConfigJSON, err := secrets.CreateRegistrySecretDockerConfigJSON(rbrs.registryConfig)
	assert.NoError(rbrs.T(), err)
	secretData := map[string][]byte{
		corev1.DockerConfigJsonKey: []byte(dockerConfigJSON),
	}

	for _, tt := range tests {
		subSession := rbrs.session.NewSession()
		defer subSession.Cleanup()

		rbrs.Run("Validate listing registry secret for user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the projects.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbrs.client, rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbrs.client, tt.member, tt.role.String(), rbrs.cluster, adminProject)
			assert.NoError(rbrs.T(), err)
			rbrs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a registry secret in the namespace %v", rbac.Admin, namespace.Name)
			createdRegistrySecret, err := secrets.CreateSecret(rbrs.client, rbrs.cluster.ID, namespace.Name, secretData, corev1.SecretTypeDockerConfigJson)
			assert.NoError(rbrs.T(), err, "failed to create a registry secret")

			log.Infof("As a %v, list the registry secrets.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)
			secretList, err := standardUserContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbrs.T(), err, "failed to list registry secret")
				assert.Equal(rbrs.T(), len(secretList.Items), 1)
				assert.Equal(rbrs.T(), secretList.Items[0].Name, createdRegistrySecret.Name)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbrs.T(), err)
				assert.True(rbrs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbrs *RbacRegistrySecretTestSuite) TestUpdateRegistrySecret() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	dockerConfigJSON, err := secrets.CreateRegistrySecretDockerConfigJSON(rbrs.registryConfig)
	assert.NoError(rbrs.T(), err)
	secretData := map[string][]byte{
		corev1.DockerConfigJsonKey: []byte(dockerConfigJSON),
	}

	for _, tt := range tests {
		subSession := rbrs.session.NewSession()
		defer subSession.Cleanup()

		rbrs.Run("Validate updating secret as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the projects.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbrs.client, rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbrs.client, tt.member, tt.role.String(), rbrs.cluster, adminProject)
			assert.NoError(rbrs.T(), err)
			rbrs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a registry secret in the namespace %v", rbac.Admin, namespace.Name)
			createdRegistrySecret, err := secrets.CreateSecret(rbrs.client, rbrs.cluster.ID, namespace.Name, secretData, corev1.SecretTypeDockerConfigJson)
			assert.NoError(rbrs.T(), err, "failed to create a registry secret")

			log.Infof("As a %v, update the registry secret with a new label.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)

			if createdRegistrySecret.Labels == nil {
				createdRegistrySecret.Labels = make(map[string]string)
			}
			createdRegistrySecret.Labels["dummy"] = "true"

			updatedRegistrySecret, err := standardUserContext.Core.Secret().Update(createdRegistrySecret)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbrs.T(), err, "failed to update registry secret")
				assert.Equal(rbrs.T(), "true", updatedRegistrySecret.Labels["dummy"], "registry secret label was not updated")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbrs.T(), err)
				assert.True(rbrs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rbrs *RbacRegistrySecretTestSuite) TestDeleteRegistrySecret() {
	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	dockerConfigJSON, err := secrets.CreateRegistrySecretDockerConfigJSON(rbrs.registryConfig)
	assert.NoError(rbrs.T(), err)
	secretData := map[string][]byte{
		corev1.DockerConfigJsonKey: []byte(dockerConfigJSON),
	}

	for _, tt := range tests {
		subSession := rbrs.session.NewSession()
		defer subSession.Cleanup()

		rbrs.Run("Validate deleting registry secret as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the projects.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rbrs.client, rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(rbrs.client, tt.member, tt.role.String(), rbrs.cluster, adminProject)
			assert.NoError(rbrs.T(), err)
			rbrs.T().Logf("Created user: %v", newUser.Username)

			log.Infof("As a %v, create a registry secret in the namespace %v", rbac.Admin, namespace.Name)
			createdRegistrySecret, err := secrets.CreateSecret(rbrs.client, rbrs.cluster.ID, namespace.Name, secretData, corev1.SecretTypeDockerConfigJson)
			assert.NoError(rbrs.T(), err, "failed to create a registry secret")

			log.Infof("As a %v, delete the registry secrets.", tt.role.String())
			standardUserContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(rbrs.cluster.ID)
			assert.NoError(rbrs.T(), err)

			err = standardUserContext.Core.Secret().Delete(namespace.Name, createdRegistrySecret.Name, &metav1.DeleteOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rbrs.T(), err, "failed to delete registry secret")
				secretList, err := rbrs.client.WranglerContext.Core.Secret().List(namespace.Name, metav1.ListOptions{})
				assert.NoError(rbrs.T(), err)
				assert.Equal(rbrs.T(), len(secretList.Items), 0)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rbrs.T(), err)
				assert.True(rbrs.T(), errors.IsForbidden(err))
			}
		})
	}
}

func TestRbacRegistrySecretTestSuite(t *testing.T) {
	suite.Run(t, new(RbacRegistrySecretTestSuite))
}
