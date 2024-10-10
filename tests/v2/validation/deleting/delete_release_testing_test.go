//go:build validation

package deleting

import (
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/qaseinput"
	qase "github.com/rancher/rancher/tests/v2/validation/pipeline/qase/results"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type DeleteReleaseTestingTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
	qaseConfig         *qaseinput.Config
}

func (d *DeleteReleaseTestingTestSuite) TearDownSuite() {
	d.session.Cleanup()

	d.qaseConfig = new(qaseinput.Config)
	config.LoadConfig(qaseinput.ConfigurationFileKey, d.qaseConfig)

	if d.qaseConfig.LocalQaseReporting {
		err := qase.ReportTest()
		require.NoError(d.T(), err)
	}
}

func (d *DeleteReleaseTestingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	d.session = testSession

	d.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, d.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(d.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(d.T(), err)

	d.standardUserClient = standardUserClient
}

func (d *DeleteReleaseTestingTestSuite) TestDeleteRKE1Cluster() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Deleting RKE1 Cluster", d.standardUserClient},
	}

	for _, tt := range tests {
		subSession := d.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(d.T(), err)

		provisioningConfig := *d.provisioningConfig
		provisioningConfig.NodePools = nodeRolesDedicated
		_, clusterObject := permutations.RunTestPermutations(&d.Suite, tt.name, client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(d.T(), err)

		clusterID, err := clusters.GetClusterIDByName(adminClient, clusterObject.Name)
		require.NoError(d.T(), err)

		clusters.DeleteRKE1Cluster(client, clusterID)
		provisioning.VerifyDeleteRKE1Cluster(d.T(), client, clusterID)
	}
}

func (d *DeleteReleaseTestingTestSuite) TestDeleteCluster() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Deleting RKE2 Cluster", d.standardUserClient},
		{"Deleting K3S Cluster", d.standardUserClient},
	}

	for _, tt := range tests {
		subSession := d.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(d.T(), err)

		var clusterObject *v1.SteveAPIObject
		provisioningConfig := *d.provisioningConfig
		provisioningConfig.MachinePools = nodeRolesDedicated

		if strings.Contains(tt.name, "RKE2") {
			clusterObject, _ = permutations.RunTestPermutations(&d.Suite, tt.name, client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
		} else {
			clusterObject, _ = permutations.RunTestPermutations(&d.Suite, tt.name, client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(d.T(), err)

		clusterID, err := clusters.GetV1ProvisioningClusterByName(adminClient, clusterObject.Name)
		require.NoError(d.T(), err)

		clusters.DeleteK3SRKE2Cluster(adminClient, clusterID)
		provisioning.VerifyDeleteRKE2K3SCluster(d.T(), adminClient, clusterID)
	}
}

func (d *DeleteReleaseTestingTestSuite) TestDeleteInitMachine() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"RKE2", d.standardUserClient},
		{"K3S", d.standardUserClient},
	}

	for _, tt := range tests {
		subSession := d.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(d.T(), err)

		var clusterObject *v1.SteveAPIObject
		provisioningConfig := *d.provisioningConfig
		provisioningConfig.MachinePools = nodeRolesDedicated

		if strings.Contains(tt.name, "RKE2") {
			clusterObject, _ = permutations.RunTestPermutations(&d.Suite, tt.name, client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
		} else {
			clusterObject, _ = permutations.RunTestPermutations(&d.Suite, tt.name, client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
		}

		adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
		require.NoError(d.T(), err)

		clusterID, err := clusters.GetV1ProvisioningClusterByName(adminClient, clusterObject.Name)
		require.NoError(d.T(), err)

		deleteInitMachine(d.T(), adminClient, clusterID)
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDeleteReleaseTestingTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteReleaseTestingTestSuite))
}
