//go:build (validation || infra.rke1 || cluster.nodedriver || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !sanity

package rbac

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/clustertemplates"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	mgmt "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/settings"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/config"

	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	dockerDir = "/var/lib/docker"
)

type ClusterTemplateRKE1RBACTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
}

func (ct *ClusterTemplateRKE1RBACTestSuite) TearDownSuite() {
	ct.session.Cleanup()
}

func (ct *ClusterTemplateRKE1RBACTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ct.session = testSession

	ct.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, ct.provisioningConfig)

	client, err := rancher.NewClient("", testSession) //ct.client
	require.NoError(ct.T(), err)

	ct.client = client

	if ct.provisioningConfig.RKE1KubernetesVersions == nil {
		rke1Versions, err := kubernetesversions.ListRKE1AllVersions(ct.client)
		require.NoError(ct.T(), err)

		ct.provisioningConfig.RKE1KubernetesVersions = []string{rke1Versions[len(rke1Versions)-1]}
	}

	if ct.provisioningConfig.CNIs == nil {
		ct.provisioningConfig.CNIs = []string{clustertemplates.CniCalico}
	}
}

func (ct *ClusterTemplateRKE1RBACTestSuite) TestClusterTemplateEnforcementForStandardUser() {
	log.Info("Enforcing cluster template while provisioning rke1 clusters")

	clusterEnforcement, err := ct.client.Steve.SteveType(settings.ManagementSetting).ByID(clustertemplates.ClusterEnforcementSetting)
	require.NoError(ct.T(), err)
	_, err = settings.UpdateGlobalSettings(ct.client.Steve, clusterEnforcement, clustertemplates.EnabledClusterEnforcementSetting)
	require.NoError(ct.T(), err)

	verifySetting, err := ct.client.Steve.SteveType(settings.ManagementSetting).ByID(clustertemplates.ClusterEnforcementSetting)
	require.NoError(ct.T(), err)
	require.Equal(ct.T(), verifySetting.JSONResp["value"], clustertemplates.EnabledClusterEnforcementSetting)

	log.Info("Creating a cluster template and verifying the user is not able to create the cluster without the required permissions.")
	clusterTemplate, err := clustertemplates.CreateRkeTemplate(ct.client, nil)
	require.NoError(ct.T(), err)

	templateRevisonConfig := mgmt.ClusterTemplateRevision{
		ClusterConfig: &mgmt.ClusterSpecBase{RancherKubernetesEngineConfig: &mgmt.RancherKubernetesEngineConfig{
			Network: &mgmt.NetworkConfig{Plugin: ct.provisioningConfig.CNIs[0]},
			Version: ct.provisioningConfig.RKE1KubernetesVersions[0],
		},
		},
	}

	clusterTemplateRevision, err := clustertemplates.CreateRkeTemplateRevision(ct.client, templateRevisonConfig, clusterTemplate.ID)
	require.NoError(ct.T(), err)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}

	log.Info("Create a downstream cluster as the standard user.")

	log.Info("Create a standard user.")
	user, err := users.CreateUserWithRole(ct.client, users.UserConfig(), "user")
	require.NoError(ct.T(), err)
	standardClient, err := ct.client.AsUser(user)
	require.NoError(ct.T(), err)

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(standardClient)
	require.NoError(ct.T(), err)

	_, err = provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(standardClient, clusterTemplate.ID, clusterTemplateRevision.ID,
		nodeAndRoles, nodeTemplate, nil)
	require.Error(ct.T(), err, "User is not expected to be able to create a cluster.")
	require.Contains(ct.T(), err.Error(), "The clusterTemplateRevision is not found")

	log.Info("Verifying user can now create the cluster with the permissions added to the cluster template.")

	members := []mgmt.Member{
		0: {AccessType: "owner",
			UserPrincipalID: clustertemplates.UserPrincipalID + user.ID},
	}

	updatedMembersTemplate := *clusterTemplate
	updatedMembersTemplate.Members = members

	_, err = ct.client.Management.ClusterTemplate.Update(clusterTemplate, updatedMembersTemplate)
	require.NoError(ct.T(), err)

	clusterObj, err := provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(standardClient, clusterTemplate.ID, clusterTemplateRevision.ID,
		nodeAndRoles, nodeTemplate, nil)
	require.NoError(ct.T(), err)

	log.Info("Verifying the rke1 cluster comes up active.")

	clusterConfig := clusters.ConvertConfigToClusterConfig(ct.provisioningConfig)
	clusterConfig.KubernetesVersion = ct.provisioningConfig.RKE1KubernetesVersions[0]
	provisioning.VerifyRKE1Cluster(ct.T(), ct.client, clusterConfig, clusterObj)

	log.Info("Update the global setting to enforce rke templates back to false.")

	clusterEnforcement, err = ct.client.Steve.SteveType(settings.ManagementSetting).ByID(clustertemplates.ClusterEnforcementSetting)
	require.NoError(ct.T(), err)
	_, err = settings.UpdateGlobalSettings(ct.client.Steve, clusterEnforcement, clustertemplates.DisabledClusterEnforcementSetting)
	require.NoError(ct.T(), err)

	verifySetting, err = ct.client.Steve.SteveType(settings.ManagementSetting).ByID(clustertemplates.ClusterEnforcementSetting)
	require.NoError(ct.T(), err)
	require.Equal(ct.T(), verifySetting.JSONResp["value"], clustertemplates.DisabledClusterEnforcementSetting)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterTemplateRKE1RBACTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterTemplateRKE1RBACTestSuite))
}
