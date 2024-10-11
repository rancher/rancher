//go:build (validation || infra.any || cluster.any || stress) && !sanity && !extended

package snapshotrbac

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/projects"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/etcdsnapshot"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SnapshotRBACTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (etcd *SnapshotRBACTestSuite) TearDownSuite() {
	etcd.session.Cleanup()
}

func (etcd *SnapshotRBACTestSuite) SetupSuite() {
	etcd.session = session.NewSession()

	client, err := rancher.NewClient("", etcd.session)
	require.NoError(etcd.T(), err)

	etcd.client = client
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(etcd.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(etcd.client, clusterName)
	require.NoError(etcd.T(), err, "Error getting cluster ID")
	etcd.cluster, err = etcd.client.Management.Cluster.ByID(clusterID)
	require.NoError(etcd.T(), err)
}

func (etcd *SnapshotRBACTestSuite) testRKE2K3SSnapshotRBAC(role string, standardUserClient *rancher.Client) {
	log.Info("Test case - Take Etcd snapshot of a cluster as a " + role)
	_, err := etcdsnapshot.CreateRKE2K3SSnapshot(standardUserClient, etcd.cluster.Name)
	switch role {
	case rbac.ClusterOwner.String(), rbac.RestrictedAdmin.String():
		require.NoError(etcd.T(), err)
		log.Info("Snapshot successful!")

	case rbac.ClusterMember.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
		require.Error(etcd.T(), err)
		assert.Equal(etcd.T(), "Resource type [provisioning.cattle.io.cluster] is not updatable", err.Error())
		log.Info("Snapshot failed as expected.")
	}
}

func (etcd *SnapshotRBACTestSuite) testRKE1SnapshotRBAC(role string, standardUserClient *rancher.Client) {
	log.Info("Test case - Take Etcd snapshot of an RKE1 cluster as a " + role)
	_, err := etcdsnapshot.CreateRKE1Snapshot(standardUserClient, etcd.cluster.Name)
	switch role {
	case rbac.ClusterOwner.String(), rbac.RestrictedAdmin.String():
		require.NoError(etcd.T(), err)
		log.Info("Snapshot successful!")

	case rbac.ClusterMember.String(), rbac.ProjectOwner.String(), rbac.ProjectMember.String():
		require.Error(etcd.T(), err)
		assert.Contains(etcd.T(), err.Error(), "action [backupEtcd] not available")
		log.Info("Snapshot failed as expected.")
	}
}

func (etcd *SnapshotRBACTestSuite) TestETCDRBAC() {
	tests := []struct {
		name   string
		role   string
		member string
	}{
		{"Cluster Owner", rbac.ClusterOwner.String(), rbac.StandardUser.String()},
		{"Cluster Member", rbac.ClusterMember.String(), rbac.StandardUser.String()},
		{"Project Owner", rbac.ProjectOwner.String(), rbac.StandardUser.String()},
		{"Project Member", rbac.ProjectMember.String(), rbac.StandardUser.String()},
		{"Restricted Admin", rbac.RestrictedAdmin.String(), rbac.RestrictedAdmin.String()},
	}
	for _, tt := range tests {
		etcd.Run("Set up User with Role "+tt.name, func() {
			clusterUser, clusterClient, err := rbac.SetupUser(etcd.client, tt.member)
			require.NoError(etcd.T(), err)

			adminProject, err := etcd.client.Management.Project.Create(projects.NewProjectConfig(etcd.cluster.ID))
			require.NoError(etcd.T(), err)

			if tt.member == rbac.StandardUser.String() {
				if strings.Contains(tt.role, "project") {
					err := users.AddProjectMember(etcd.client, adminProject, clusterUser, tt.role, nil)
					require.NoError(etcd.T(), err)
				} else {
					err := users.AddClusterRoleToUser(etcd.client, etcd.cluster, clusterUser, tt.role, nil)
					require.NoError(etcd.T(), err)
				}
			}

			relogin, err := clusterClient.ReLogin()
			require.NoError(etcd.T(), err)
			clusterClient = relogin

			if !(strings.Contains(etcd.cluster.ID, "c-m-")) {
				etcd.testRKE1SnapshotRBAC(tt.role, clusterClient)
			} else {
				etcd.testRKE2K3SSnapshotRBAC(tt.role, clusterClient)
			}
			subSession := etcd.session.NewSession()
			defer subSession.Cleanup()
		})
	}
}

func TestSnapshotRBACTestSuite(t *testing.T) {
	suite.Run(t, new(SnapshotRBACTestSuite))
}
