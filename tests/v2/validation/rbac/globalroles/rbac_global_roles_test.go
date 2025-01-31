//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package globalroles

import (
	"testing"

	rbacapi "github.com/rancher/rancher/tests/v2/actions/kubeapi/rbac"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
)

type RbacGlobalRolesTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (rgr *RbacGlobalRolesTestSuite) TearDownSuite() {
	rgr.session.Cleanup()
}

func (rgr *RbacGlobalRolesTestSuite) SetupSuite() {
	rgr.session = session.NewSession()

	client, err := rancher.NewClient("", rgr.session)
	require.NoError(rgr.T(), err)
	rgr.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rgr")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rgr.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rgr.client, clusterName)
	require.NoError(rgr.T(), err, "Error getting cluster ID")
	rgr.cluster, err = rgr.client.Management.Cluster.ByID(clusterID)
	require.NoError(rgr.T(), err)
}

func (rgr *RbacGlobalRolesTestSuite) TestCreateGlobalRole() {
	subSession := rgr.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.Admin, rbac.Admin.String()},
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rgr.Run("Validate global role creation with role "+tt.role.String(), func() {
			switch tt.role.String() {
			case rbac.Admin.String():
				log.Infof("As a %v, create a global role", tt.role.String())
				_, err := createCustomGlobalRole(rgr.client)
				assert.NoError(rgr.T(), err)
			case rbac.ClusterOwner.String(), rbac.ClusterMember.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				log.Info("Create a project and a namespace in the project.")
				adminProject, _, err := projects.CreateProjectAndNamespace(rgr.client, rgr.cluster.ID)
				assert.NoError(rgr.T(), err)

				log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
				_, userClient, err := rbac.AddUserWithRoleToCluster(rgr.client, tt.member, tt.role.String(), rgr.cluster, adminProject)
				assert.NoError(rgr.T(), err)

				log.Infof("As a %v, create a global role", tt.role.String())
				_, err = createCustomGlobalRole(userClient)
				assert.Error(rgr.T(), err)
				assert.True(rgr.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rgr *RbacGlobalRolesTestSuite) TestListGlobalRole() {
	subSession := rgr.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.Admin, rbac.Admin.String()},
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rgr.Run("Validate listing global role with role "+tt.role.String(), func() {
			log.Infof("As a admin, create a global role")
			createdGlobalRole, err := createCustomGlobalRole(rgr.client)
			assert.NoError(rgr.T(), err)

			switch tt.role.String() {
			case rbac.Admin.String():
				log.Infof("As a %v, list the global role", tt.role.String())
				grole, err := rbac.GetGlobalRoleByName(rgr.client, createdGlobalRole.Name)
				assert.NoError(rgr.T(), err)
				assert.Equal(rgr.T(), grole.Name, createdGlobalRole.Name)
			case rbac.ClusterOwner.String(), rbac.ClusterMember.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				log.Info("Create a project and a namespace in the project.")
				adminProject, _, err := projects.CreateProjectAndNamespace(rgr.client, rgr.cluster.ID)
				assert.NoError(rgr.T(), err)

				log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
				_, userClient, err := rbac.AddUserWithRoleToCluster(rgr.client, tt.member, tt.role.String(), rgr.cluster, adminProject)
				assert.NoError(rgr.T(), err)

				log.Infof("As a %v, list the global role", tt.role.String())
				_, err = rbac.GetGlobalRoleByName(userClient, createdGlobalRole.Name)
				assert.Error(rgr.T(), err)
				assert.True(rgr.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rgr *RbacGlobalRolesTestSuite) TestUpdateGlobalRole() {
	subSession := rgr.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.Admin, rbac.Admin.String()},
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rgr.Run("Validate updating a global role with role "+tt.role.String(), func() {
			log.Infof("As a admin, create a global role")
			createdGlobalRole, err := createCustomGlobalRole(rgr.client)
			assert.NoError(rgr.T(), err)

			globalRole, err := rbac.GetGlobalRoleByName(rgr.client, createdGlobalRole.Name)
			assert.NoError(rgr.T(), err)

			if globalRole.Labels == nil {
				globalRole.Labels = make(map[string]string)
			}
			globalRole.Labels["test-label"] = "true"

			switch tt.role.String() {
			case rbac.Admin.String():
				log.Infof("As a %v, update the global role", tt.role.String())
				_, err = rbacapi.UpdateGlobalRole(rgr.client, globalRole)
				assert.NoError(rgr.T(), err)

				updatedGlobalRole, err := rbac.GetGlobalRoleByName(rgr.client, createdGlobalRole.Name)
				assert.NoError(rgr.T(), err)
				assert.Equal(rgr.T(), "true", updatedGlobalRole.Labels["test-label"], "Label not updated as expected")
			case rbac.ClusterOwner.String(), rbac.ClusterMember.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				log.Info("Create a project and a namespace in the project.")
				adminProject, _, err := projects.CreateProjectAndNamespace(rgr.client, rgr.cluster.ID)
				assert.NoError(rgr.T(), err)

				log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
				_, userClient, err := rbac.AddUserWithRoleToCluster(rgr.client, tt.member, tt.role.String(), rgr.cluster, adminProject)
				assert.NoError(rgr.T(), err)

				log.Infof("As a %v, update the global role", tt.role.String())
				_, err = rbacapi.UpdateGlobalRole(userClient, globalRole)
				assert.Error(rgr.T(), err)
				assert.True(rgr.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rgr *RbacGlobalRolesTestSuite) TestDeleteGlobalRole() {
	subSession := rgr.session.NewSession()
	defer subSession.Cleanup()

	tests := []struct {
		role   rbac.Role
		member string
	}{
		{rbac.Admin, rbac.Admin.String()},
		{rbac.ClusterOwner, rbac.StandardUser.String()},
		{rbac.ClusterMember, rbac.StandardUser.String()},
		{rbac.ProjectOwner, rbac.StandardUser.String()},
		{rbac.ProjectMember, rbac.StandardUser.String()},
		{rbac.ReadOnly, rbac.StandardUser.String()},
	}

	for _, tt := range tests {
		rgr.Run("Validate deleting a global role with role "+tt.role.String(), func() {
			log.Infof("As a admin, create a global role")
			createdGlobalRole, err := createCustomGlobalRole(rgr.client)
			assert.NoError(rgr.T(), err)

			switch tt.role.String() {
			case rbac.Admin.String():
				log.Infof("As a %v, delete the global role", tt.role.String())
				err = rbacapi.DeleteGlobalRole(rgr.client, createdGlobalRole.Name)
				assert.NoError(rgr.T(), err)

				_, err = rbac.GetGlobalRoleByName(rgr.client, createdGlobalRole.Name)
				assert.Error(rgr.T(), err)
			case rbac.ClusterOwner.String(), rbac.ClusterMember.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				log.Info("Create a project and a namespace in the project.")
				adminProject, _, err := projects.CreateProjectAndNamespace(rgr.client, rgr.cluster.ID)
				assert.NoError(rgr.T(), err)

				log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
				_, userClient, err := rbac.AddUserWithRoleToCluster(rgr.client, tt.member, tt.role.String(), rgr.cluster, adminProject)
				assert.NoError(rgr.T(), err)

				log.Infof("As a %v, delete the global role", tt.role.String())
				err = rbacapi.DeleteGlobalRole(userClient, createdGlobalRole.Name)
				assert.Error(rgr.T(), err)
				assert.True(rgr.T(), errors.IsForbidden(err))
			}
		})
	}
}

func TestRbacGlobalRolesTestSuite(t *testing.T) {
	suite.Run(t, new(RbacGlobalRolesTestSuite))
}
