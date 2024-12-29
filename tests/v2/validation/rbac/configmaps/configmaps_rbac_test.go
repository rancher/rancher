//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package configmaps

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/configmaps"
	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	deployment "github.com/rancher/rancher/tests/v2/actions/workloads/deployment"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/shepherd/pkg/wrangler"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	deploymentName = namegen.AppendRandomString("testcm-dep-")
	data           = map[string]string{"foo": "bar"}
)

type ConfigmapsRBACTestSuite struct {
	suite.Suite
	client     *rancher.Client
	session    *session.Session
	cluster    *management.Cluster
	ctxAsAdmin *wrangler.Context
}

func (cm *ConfigmapsRBACTestSuite) TearDownSuite() {
	cm.session.Cleanup()
}

func (cm *ConfigmapsRBACTestSuite) SetupSuite() {
	cm.session = session.NewSession()

	client, err := rancher.NewClient("", cm.session)
	require.NoError(cm.T(), err)
	cm.client = client

	log.Info("Getting cluster name from the config file and append cluster details in cm")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(cm.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(cm.client, clusterName)
	require.NoError(cm.T(), err, "Error getting cluster ID")
	cm.cluster, err = cm.client.Management.Cluster.ByID(clusterID)
	require.NoError(cm.T(), err)

	cm.ctxAsAdmin, err = cm.client.WranglerContext.DownStreamClusterWranglerContext(clusterID)
	require.NoError(cm.T(), err)
}

func (cm *ConfigmapsRBACTestSuite) TestCreateConfigmapAsVolume() {
	subSession := cm.session.NewSession()
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
		cm.Run("Validate config map creation for user with role "+tt.role.String(), func() {
			adminProject, namespace, err := projects.CreateProjectAndNamespace(cm.client, cm.cluster.ID)
			require.NoError(cm.T(), err)

			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(cm.client, tt.member, tt.role.String(), cm.cluster, adminProject)
			require.NoError(cm.T(), err)
			cm.T().Logf("Created user: %v", newUser.Username)

			configMapCreatedByUser, err := configmaps.CreateConfigmap(namespace.Name, standardUserClient, data, cm.cluster.ID)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				require.NoError(cm.T(), err)
				_, err = deployment.CreateDeployment(standardUserClient, cm.cluster.ID, namespace.Name, 1, "", configMapCreatedByUser.Name, false, true, false, true)
				require.NoError(cm.T(), err)
				getConfigMapAsAdmin, err := cm.ctxAsAdmin.Core.ConfigMap().Get(namespace.Name, configMapCreatedByUser.Name, metav1.GetOptions{})
				require.NoError(cm.T(), err)
				require.Equal(cm.T(), getConfigMapAsAdmin.Data, configMapCreatedByUser.Data)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				require.Error(cm.T(), err)
				require.True(cm.T(), k8sError.IsForbidden(err))
			}
		})
	}
}

func (cm *ConfigmapsRBACTestSuite) TestCreateConfigmapAsEnvVar() {
	subSession := cm.session.NewSession()
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
		cm.Run("Validate config map creation of config map and verify adding it as a an env variable for user with role "+tt.role.String(), func() {
			adminProject, namespace, err := projects.CreateProjectAndNamespace(cm.client, cm.cluster.ID)
			require.NoError(cm.T(), err)

			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(cm.client, tt.member, tt.role.String(), cm.cluster, adminProject)
			require.NoError(cm.T(), err)
			cm.T().Logf("Created user: %v", newUser.Username)

			configMapCreatedByUser, err := configmaps.CreateConfigmap(namespace.Name, standardUserClient, data, cm.cluster.ID)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				require.NoError(cm.T(), err)
				_, err = deployment.CreateDeployment(standardUserClient, cm.cluster.ID, namespace.Name, 1, "", configMapCreatedByUser.Name, true, false, false, true)
				require.NoError(cm.T(), err)
				getConfigMapAsAdmin, err := cm.ctxAsAdmin.Core.ConfigMap().Get(namespace.Name, configMapCreatedByUser.Name, metav1.GetOptions{})
				require.NoError(cm.T(), err)
				require.Equal(cm.T(), getConfigMapAsAdmin.Data, configMapCreatedByUser.Data)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				require.Error(cm.T(), err)
				require.True(cm.T(), k8sError.IsForbidden(err))
			}
		})
	}
}

func (cm *ConfigmapsRBACTestSuite) TestUpdateConfigmap() {
	subSession := cm.session.NewSession()
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
		cm.Run("Validate updating config map for user with role "+tt.role.String(), func() {
			adminProject, namespace, err := projects.CreateProjectAndNamespace(cm.client, cm.cluster.ID)
			require.NoError(cm.T(), err)

			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(cm.client, tt.member, tt.role.String(), cm.cluster, adminProject)
			require.NoError(cm.T(), err)
			cm.T().Logf("Created user: %v", newUser.Username)

			configmapCreate, err := configmaps.CreateConfigmap(namespace.Name, cm.client, data, cm.cluster.ID)
			require.NoError(cm.T(), err)

			configmapCreate.Data["foo1"] = "bar1"
			userDownstreamWranglerContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(cm.cluster.ID)
			require.NoError(cm.T(), err)
			adminDownstreamWranglerContext, err := cm.client.WranglerContext.DownStreamClusterWranglerContext(cm.cluster.ID)
			require.NoError(cm.T(), err)
			configMapUpdatedByUser, userErr := userDownstreamWranglerContext.Core.ConfigMap().Update(configmapCreate)

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				require.NoError(cm.T(), userErr)
				_, err = deployment.CreateDeployment(cm.client, cm.cluster.ID, namespace.Name, 1, "", configmapCreate.Name, true, false, false, true)
				require.NoError(cm.T(), err)
				getCMAsAdmin, err := adminDownstreamWranglerContext.Core.ConfigMap().Get(namespace.Name, configMapUpdatedByUser.Name, metav1.GetOptions{})
				require.NoError(cm.T(), err)
				assert.Equal(cm.T(), configMapUpdatedByUser.Data, getCMAsAdmin.Data)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				require.Error(cm.T(), userErr)
				require.True(cm.T(), k8sError.IsForbidden(userErr))
			}
		})
	}
}

func (cm *ConfigmapsRBACTestSuite) TestListConfigmaps() {
	subSession := cm.session.NewSession()
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
		cm.Run("Validate listing config maps for user with role "+tt.role.String(), func() {
			adminProject, namespace, err := projects.CreateProjectAndNamespace(cm.client, cm.cluster.ID)
			require.NoError(cm.T(), err)

			newUser, standardUserClient, err := rbac.AddUserWithRoleToCluster(cm.client, tt.member, tt.role.String(), cm.cluster, adminProject)
			require.NoError(cm.T(), err)
			cm.T().Logf("Created user: %v", newUser.Username)

			configMapCreatedByAdmin, err := configmaps.CreateConfigmap(namespace.Name, cm.client, data, cm.cluster.ID)
			require.NoError(cm.T(), err)

			downstreamWranglerContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(cm.cluster.ID)
			configMapListAsUser, err := downstreamWranglerContext.Core.ConfigMap().List(namespace.Name, metav1.ListOptions{
				FieldSelector: "metadata.name=" + configMapCreatedByAdmin.Name,
			})

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				require.NoError(cm.T(), err)
				require.Equal(cm.T(), len(configMapListAsUser.Items), 1)
			case rbac.ClusterMember.String():
				require.Error(cm.T(), err)
				require.True(cm.T(), k8sError.IsForbidden(err))
			}
		})
	}
}

func (cm *ConfigmapsRBACTestSuite) TestDeleteConfigmap() {
	subSession := cm.session.NewSession()
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
		cm.Run("Validate deletion of config map for user with role "+tt.role.String(), func() {
			adminProject, namespace, err := projects.CreateProjectAndNamespace(cm.client, cm.cluster.ID)
			require.NoError(cm.T(), err)

			_, standardUserClient, err := rbac.AddUserWithRoleToCluster(cm.client, tt.member, tt.role.String(), cm.cluster, adminProject)
			require.NoError(cm.T(), err)

			configMapCreatedByAdmin, err := configmaps.CreateConfigmap(namespace.Name, cm.client, data, cm.cluster.ID)
			require.NoError(cm.T(), err)

			userDownstreamWranglerContext, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(cm.cluster.ID)
			require.NoError(cm.T(), err)
			err = userDownstreamWranglerContext.Core.ConfigMap().Delete(namespace.Name, configMapCreatedByAdmin.Name, &metav1.DeleteOptions{})
			configMapListAsAdmin, errList := cm.ctxAsAdmin.Core.ConfigMap().List(namespace.Name, metav1.ListOptions{
				FieldSelector: "metadata.name=" + configMapCreatedByAdmin.Name,
			})
			require.NoError(cm.T(), errList)

			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				require.NoError(cm.T(), err)
				require.Equal(cm.T(), len(configMapListAsAdmin.Items), 0)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				require.Error(cm.T(), err)
				require.True(cm.T(), k8sError.IsForbidden(err))
				require.Equal(cm.T(), len(configMapListAsAdmin.Items), 1)
			}
		})
	}
}

func (cm *ConfigmapsRBACTestSuite) TestCRUDConfigmapAsClusterMember() {
	subSession := cm.session.NewSession()
	defer subSession.Cleanup()

	log.Info("Creating a standard user and adding them to cluster as a cluster member.")
	_, standardUserClient, err := rbac.AddUserWithRoleToCluster(cm.client, rbac.StandardUser.String(), rbac.ClusterMember.String(), cm.cluster, nil)
	require.NoError(cm.T(), err)

	_, namespace, err := projects.CreateProjectAndNamespace(standardUserClient, cm.cluster.ID)
	require.NoError(cm.T(), err)

	configMapCreatedByAdmin, err := configmaps.CreateConfigmap(namespace.Name, cm.client, data, cm.cluster.ID)
	require.NoError(cm.T(), err)

	downstreamWranglerContextAsClusterMember, err := standardUserClient.WranglerContext.DownStreamClusterWranglerContext(cm.cluster.ID)
	require.NoError(cm.T(), err)

	cm.Run("Validate config map creation in the project created by "+rbac.ClusterMember.String(), func() {
		configMapCreatedByClusterMember, err := configmaps.CreateConfigmap(namespace.Name, standardUserClient, data, cm.cluster.ID)
		require.NoError(cm.T(), err)

		_, err = deployment.CreateDeployment(standardUserClient, cm.cluster.ID, namespace.Name, 1, "", configMapCreatedByClusterMember.Name, true, false, false, true)

		require.NoError(cm.T(), err)
	})

	cm.Run("Validate cluster member can update admin created config map in the project created by cluster member.", func() {
		configMapCreatedByAdmin.Data["foo1"] = "bar1"
		require.NoError(cm.T(), err)

		configMapCreatedByUser, err := downstreamWranglerContextAsClusterMember.Core.ConfigMap().Update(configMapCreatedByAdmin)
		require.NoError(cm.T(), err)

		configmapListAsAdmin, err := cm.ctxAsAdmin.Core.ConfigMap().List(namespace.Namespace, metav1.ListOptions{
			FieldSelector: "metadata.name=" + configMapCreatedByUser.Name,
		})

		require.Equal(cm.T(), len(configmapListAsAdmin.Items), 1)
		require.Equal(cm.T(), configMapCreatedByUser.Data, configmapListAsAdmin.Items[0].Data)
	})

	cm.Run("Validate cluster member can list config maps from the project created by cluster member.", func() {

		configMapListAsClusterMember, err := downstreamWranglerContextAsClusterMember.Core.ConfigMap().List(namespace.Name, metav1.ListOptions{})
		require.NoError(cm.T(), err)

		configMapListAsAdmin, err := cm.ctxAsAdmin.Core.ConfigMap().List(namespace.Name, metav1.ListOptions{})
		require.NoError(cm.T(), err)

		require.Equal(cm.T(), len(configMapListAsAdmin.Items), len(configMapListAsClusterMember.Items))
		require.Equal(cm.T(), configMapListAsClusterMember, configMapListAsAdmin)
	})

	cm.Run("Validate cluster member can delete config maps from the project.", func() {

		err = downstreamWranglerContextAsClusterMember.Core.ConfigMap().Delete(namespace.Name, configMapCreatedByAdmin.Name, &metav1.DeleteOptions{})
		require.NoError(cm.T(), err)

		configMapListAsAdmin, err := cm.ctxAsAdmin.Core.ConfigMap().List(namespace.Name, metav1.ListOptions{
			FieldSelector: "metadata.name=" + configMapCreatedByAdmin.Name,
		})
		require.NoError(cm.T(), err)
		require.Nil(cm.T(), configMapListAsAdmin.Items)
	})

}

func TestConfigmapsRBACTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigmapsRBACTestSuite))
}
