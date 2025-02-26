//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package workloads

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	"github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/rancher/tests/v2/actions/workloads/daemonset"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RbacDaemonsetTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (rds *RbacDaemonsetTestSuite) TearDownSuite() {
	rds.session.Cleanup()
}

func (rds *RbacDaemonsetTestSuite) SetupSuite() {
	rds.session = session.NewSession()

	client, err := rancher.NewClient("", rds.session)
	require.NoError(rds.T(), err)
	rds.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rds")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(rds.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(rds.client, clusterName)
	require.NoError(rds.T(), err, "Error getting cluster ID")
	rds.cluster, err = rds.client.Management.Cluster.ByID(clusterID)
	require.NoError(rds.T(), err)
}

func (rds *RbacDaemonsetTestSuite) TestCreateDaemonset() {
	subSession := rds.session.NewSession()
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
		rds.Run("Validate daemonset creation as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rds.client, rds.cluster.ID)
			assert.NoError(rds.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rds.client, tt.member, tt.role.String(), rds.cluster, adminProject)
			assert.NoError(rds.T(), err)

			log.Infof("As a %v, create a daemonset", tt.role.String())
			_, err = daemonset.CreateDaemonset(userClient, rds.cluster.ID, namespace.Name, 1, "", "", false, false, false)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rds.T(), err, "failed to create daemonset")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rds.T(), err)
				assert.True(rds.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rds *RbacDaemonsetTestSuite) TestListDaemonset() {
	subSession := rds.session.NewSession()
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
		rds.Run("Validate listing daemonset as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rds.client, rds.cluster.ID)
			assert.NoError(rds.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rds.client, tt.member, tt.role.String(), rds.cluster, adminProject)
			assert.NoError(rds.T(), err)

			log.Infof("As a %v, create a daemonset in the namespace %v", rbac.Admin, namespace.Name)
			createdDaemonset, err := daemonset.CreateDaemonset(rds.client, rds.cluster.ID, namespace.Name, 1, "", "", false, false, true)
			assert.NoError(rds.T(), err, "failed to create daemonset")

			log.Infof("As a %v, list the daemonset", tt.role.String())
			standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rds.cluster.ID)
			assert.NoError(rds.T(), err)
			daemonsetList, err := standardUserContext.Apps.DaemonSet().List(namespace.Name, metav1.ListOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String(), rbac.ReadOnly.String():
				assert.NoError(rds.T(), err, "failed to list daemonset")
				assert.Equal(rds.T(), len(daemonsetList.Items), 1)
				assert.Equal(rds.T(), daemonsetList.Items[0].Name, createdDaemonset.Name)
			case rbac.ClusterMember.String():
				assert.Error(rds.T(), err)
				assert.True(rds.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rds *RbacDaemonsetTestSuite) TestUpdateDaemonset() {
	subSession := rds.session.NewSession()
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
		rds.Run("Validate updating daemonset as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rds.client, rds.cluster.ID)
			assert.NoError(rds.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rds.client, tt.member, tt.role.String(), rds.cluster, adminProject)
			assert.NoError(rds.T(), err)

			log.Infof("As a %v, create a daemonset in the namespace %v", rbac.Admin, namespace.Name)
			createdDaemonset, err := daemonset.CreateDaemonset(rds.client, rds.cluster.ID, namespace.Name, 1, "", "", false, false, true)
			assert.NoError(rds.T(), err, "failed to create daemonset")

			log.Infof("As a %v, update the daemonSet %s with a new label.", tt.role.String(), createdDaemonset.Name)
			if createdDaemonset.Labels == nil {
				createdDaemonset.Labels = make(map[string]string)
			}
			createdDaemonset.Labels["updated"] = "true"
			updatedDaemonSet, err := daemonset.UpdateDaemonset(userClient, rds.cluster.ID, namespace.Name, createdDaemonset, false)
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rds.T(), err, "failed to update daemonset")
				standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rds.cluster.ID)
				assert.NoError(rds.T(), err)
				updatedDaemonSet, err = standardUserContext.Apps.DaemonSet().Get(namespace.Name, updatedDaemonSet.Name, metav1.GetOptions{})
				assert.NoError(rds.T(), err, "Failed to get the updated daemonSet after updating labels.")
				assert.Equal(rds.T(), "true", updatedDaemonSet.Labels["updated"], "DaemonSet label update failed.")
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rds.T(), err)
				assert.True(rds.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rds *RbacDaemonsetTestSuite) TestDeleteDaemonset() {
	subSession := rds.session.NewSession()
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
		rds.Run("Validate deleting daemonset as user with role "+tt.role.String(), func() {
			log.Info("Create a project and a namespace in the project.")
			adminProject, namespace, err := projects.CreateProjectAndNamespace(rds.client, rds.cluster.ID)
			assert.NoError(rds.T(), err)

			log.Infof("Create a standard user and add the user to a cluster/project role %s", tt.role)
			_, userClient, err := rbac.AddUserWithRoleToCluster(rds.client, tt.member, tt.role.String(), rds.cluster, adminProject)
			assert.NoError(rds.T(), err)

			log.Infof("As a %v, create a daemonset in the namespace %v", rbac.Admin, namespace.Name)
			createdDaemonset, err := daemonset.CreateDaemonset(rds.client, rds.cluster.ID, namespace.Name, 1, "", "", false, false, true)
			assert.NoError(rds.T(), err, "failed to create daemonset")

			log.Infof("As a %v, delete the daemonset", tt.role.String())
			standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rds.cluster.ID)
			assert.NoError(rds.T(), err)
			err = standardUserContext.Apps.DaemonSet().Delete(namespace.Name, createdDaemonset.Name, &metav1.DeleteOptions{})
			switch tt.role.String() {
			case rbac.ClusterOwner.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
				assert.NoError(rds.T(), err, "failed to delete daemonset")
				daemonsetList, err := standardUserContext.Apps.DaemonSet().List(namespace.Name, metav1.ListOptions{})
				assert.NoError(rds.T(), err)
				assert.Equal(rds.T(), len(daemonsetList.Items), 0)
			case rbac.ClusterMember.String(), rbac.ReadOnly.String():
				assert.Error(rds.T(), err)
				assert.True(rds.T(), errors.IsForbidden(err))
			}
		})
	}
}

func (rds *RbacDaemonsetTestSuite) TestCrudDaemonsetAsClusterMember() {
	subSession := rds.session.NewSession()
	defer subSession.Cleanup()

	role := rbac.ClusterMember.String()
	log.Infof("Create a standard user.")
	user, userClient, err := rbac.SetupUser(rds.client, rbac.StandardUser.String())
	require.NoError(rds.T(), err)

	log.Infof("Add the user to the downstream cluster with role %s", role)
	err = users.AddClusterRoleToUser(rds.client, rds.cluster, user, role, nil)
	require.NoError(rds.T(), err)

	log.Infof("As a %v, create a project and a namespace in the project.", role)
	_, namespace, err := projects.CreateProjectAndNamespace(userClient, rds.cluster.ID)
	require.NoError(rds.T(), err)

	log.Infof("As a %v, create a daemonset in the namespace %v", role, namespace.Name)
	createdDaemonset, err := daemonset.CreateDaemonset(userClient, rds.cluster.ID, namespace.Name, 1, "", "", false, false, true)
	require.NoError(rds.T(), err, "failed to create daemonset")

	log.Infof("As a %v, list the daemonset", role)
	standardUserContext, err := userClient.WranglerContext.DownStreamClusterWranglerContext(rds.cluster.ID)
	require.NoError(rds.T(), err)
	daemonsetList, err := standardUserContext.Apps.DaemonSet().List(namespace.Name, metav1.ListOptions{})
	require.NoError(rds.T(), err, "failed to list daemonset")
	require.Equal(rds.T(), len(daemonsetList.Items), 1)
	require.Equal(rds.T(), daemonsetList.Items[0].Name, createdDaemonset.Name)

	log.Infof("As a %v, update the daemonSet %s with a new label.", role, createdDaemonset.Name)
	if createdDaemonset.Labels == nil {
		createdDaemonset.Labels = make(map[string]string)
	}
	createdDaemonset.Labels["updated"] = "true"
	updatedDaemonSet, err := daemonset.UpdateDaemonset(userClient, rds.cluster.ID, namespace.Name, createdDaemonset, true)
	require.NoError(rds.T(), err, "failed to update daemonset")
	updatedDaemonSet, err = standardUserContext.Apps.DaemonSet().Get(namespace.Name, updatedDaemonSet.Name, metav1.GetOptions{})
	require.NoError(rds.T(), err, "Failed to get the updated daemonSet after updating labels.")
	require.Equal(rds.T(), "true", updatedDaemonSet.Labels["updated"], "DaemonSet label update failed.")

	log.Infof("As a %v, delete the daemonset", role)
	err = standardUserContext.Apps.DaemonSet().Delete(namespace.Name, createdDaemonset.Name, &metav1.DeleteOptions{})
	require.NoError(rds.T(), err, "failed to delete daemonset")
	daemonsetList, err = standardUserContext.Apps.DaemonSet().List(namespace.Name, metav1.ListOptions{})
	require.NoError(rds.T(), err)
	require.Equal(rds.T(), len(daemonsetList.Items), 0)
}

func TestRbacDaemonsetTestSuite(t *testing.T) {
	suite.Run(t, new(RbacDaemonsetTestSuite))
}
