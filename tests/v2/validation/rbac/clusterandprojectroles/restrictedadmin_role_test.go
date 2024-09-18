package clusterandprojectroles

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	rbac "github.com/rancher/rancher/tests/v2/actions/rbac"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/settings"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RestrictedAdminTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (ra *RestrictedAdminTestSuite) TearDownSuite() {
	ra.session.Cleanup()
}

func (ra *RestrictedAdminTestSuite) SetupSuite() {
	ra.session = session.NewSession()

	client, err := rancher.NewClient("", ra.session)
	require.NoError(ra.T(), err)

	ra.client = client

	// Disabling configuration here is to avoid interference with other pipeline tests.
	// Updates to the config are temporarily disabled during the test
	// and are automatically enabled during cleanup.
	provisioning.DisableUpdateConfig(ra.client)

	log.Info("Getting cluster name from the config file and append cluster details in the struct.")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ra.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := extensionscluster.GetClusterIDByName(ra.client, clusterName)
	require.NoError(ra.T(), err, "Error getting cluster ID")
	ra.cluster, err = ra.client.Management.Cluster.ByID(clusterID)
	require.NoError(ra.T(), err)
}

func (ra *RestrictedAdminTestSuite) TestRestrictedAdminCreateCluster() {
	subSession := ra.session.NewSession()
	defer subSession.Cleanup()

	_, restrictedAdminClient, err := rbac.SetupUser(ra.client, rbac.RestrictedAdmin.String())
	require.NoError(ra.T(), err)
	ra.T().Logf("Validating restricted admin can create a downstream cluster")
	userConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, userConfig)
	nodeProviders := userConfig.NodeProviders[0]
	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}
	externalNodeProvider := provisioning.ExternalNodeProviderSetup(nodeProviders)
	clusterConfig := clusters.ConvertConfigToClusterConfig(userConfig)
	clusterConfig.NodePools = nodeAndRoles
	kubernetesVersion, err := kubernetesversions.Default(restrictedAdminClient, extensionscluster.RKE1ClusterType.String(), []string{})
	require.NoError(ra.T(), err)

	clusterConfig.KubernetesVersion = kubernetesVersion[0]
	clusterConfig.CNI = userConfig.CNIs[0]
	clusterObject, _, err := provisioning.CreateProvisioningRKE1CustomCluster(restrictedAdminClient, &externalNodeProvider, clusterConfig)
	require.NoError(ra.T(), err)
	provisioning.VerifyRKE1Cluster(ra.T(), restrictedAdminClient, clusterConfig, clusterObject)
}

func (ra *RestrictedAdminTestSuite) TestRestrictedAdminGlobalSettings() {

	subSession := ra.session.NewSession()
	defer subSession.Cleanup()

	_, restrictedAdminClient, err := rbac.SetupUser(ra.client, rbac.RestrictedAdmin.String())
	require.NoError(ra.T(), err)
	ra.T().Log("Validating restricted Admin can list global settings")
	steveRestrictedAdminclient := restrictedAdminClient.Steve
	steveAdminClient := ra.client.Steve

	adminListSettings, err := steveAdminClient.SteveType(settings.ManagementSetting).List(nil)
	require.NoError(ra.T(), err)
	adminSettings := adminListSettings.Names()

	resAdminListSettings, err := steveRestrictedAdminclient.SteveType(settings.ManagementSetting).List(nil)
	require.NoError(ra.T(), err)
	resAdminSettings := resAdminListSettings.Names()

	assert.Equal(ra.T(), len(adminSettings), len(resAdminSettings))
	assert.Equal(ra.T(), adminSettings, resAdminSettings)
}

func (ra *RestrictedAdminTestSuite) TestRestrictedAdminCantUpdateGlobalSettings() {
	ra.T().Logf("Validating restrictedAdmin cannot edit global settings")

	_, restrictedAdminClient, err := rbac.SetupUser(ra.client, rbac.RestrictedAdmin.String())
	require.NoError(ra.T(), err)

	steveRestrictedAdminclient := restrictedAdminClient.Steve
	steveAdminClient := ra.client.Steve

	kubeConfigTokenSetting, err := steveAdminClient.SteveType(settings.ManagementSetting).ByID(settings.KubeConfigToken)
	require.NoError(ra.T(), err)

	_, err = settings.UpdateGlobalSettings(steveRestrictedAdminclient, kubeConfigTokenSetting, "3")
	require.Error(ra.T(), err)
	assert.Contains(ra.T(), err.Error(), "Resource type [management.cattle.io.setting] is not updatable")
}

func TestRestrictedAdminTestSuite(t *testing.T) {
	suite.Run(t, new(RestrictedAdminTestSuite))
}
