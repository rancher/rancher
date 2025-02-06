//go:build (validation || infra.rke1 || cluster.nodedriver || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !sanity

package clustertemplates

import (
	"strconv"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/clustertemplates"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	mgmt "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/settings"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"

	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

var (
	Questions = []mgmt.Question{
		0: {
			Variable: "rancherKubernetesEngineConfig.kubernetesVersion",
			Default:  "",
			Required: clustertemplates.IsRequiredQuestion,
			Type:     "string",
		},
		1: {
			Variable: "rancherKubernetesEngineConfig.network.plugin",
			Default:  "",
			Required: clustertemplates.IsRequiredQuestion,
			Type:     "string",
		},
		2: {
			Variable: "rancherKubernetesEngineConfig.services.etcd.backupConfig.intervalHours",
			Default:  "",
			Required: clustertemplates.IsRequiredQuestion,
			Type:     "int",
		},
	}
)

type ClusterTemplateProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
}

func (ct *ClusterTemplateProvisioningTestSuite) TearDownSuite() {
	ct.session.Cleanup()
}

func (ct *ClusterTemplateProvisioningTestSuite) SetupSuite() {
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

func (ct *ClusterTemplateProvisioningTestSuite) TestProvisioningRKE1ClusterWithClusterTemplate() {
	log.Info("Create an rke template and creating a downstream node driver with the rke template.")

	clusterTemplate, err := clustertemplates.CreateRkeTemplate(ct.client, nil)
	require.NoError(ct.T(), err)

	templateRevisonConfig := mgmt.ClusterTemplateRevision{
		ClusterConfig: &mgmt.ClusterSpecBase{
			RancherKubernetesEngineConfig: &mgmt.RancherKubernetesEngineConfig{
				Network: &mgmt.NetworkConfig{Plugin: ct.provisioningConfig.CNIs[0]},
				Version: ct.provisioningConfig.RKE1KubernetesVersions[0],
			},
		},
	}

	clusterTemplateRevision, err := clustertemplates.CreateRkeTemplateRevision(ct.client, templateRevisonConfig, clusterTemplate.ID)
	require.NoError(ct.T(), err)

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(ct.client)
	require.NoError(ct.T(), err)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}

	clusterObj, err := provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(ct.client, clusterTemplate.ID, clusterTemplateRevision.ID,
		nodeAndRoles, nodeTemplate, nil)
	require.NoError(ct.T(), err)

	log.Info("Verifying the rke1 cluster comes up active.")

	clusterConfig := clusters.ConvertConfigToClusterConfig(ct.provisioningConfig)
	clusterConfig.KubernetesVersion = ct.provisioningConfig.RKE1KubernetesVersions[0]
	provisioning.VerifyRKE1Cluster(ct.T(), ct.client, clusterConfig, clusterObj)
}

func (ct *ClusterTemplateProvisioningTestSuite) TestClusterTemplateEnforcementAsAdmin() {
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
		ClusterConfig: &mgmt.ClusterSpecBase{
			RancherKubernetesEngineConfig: &mgmt.RancherKubernetesEngineConfig{
				Network: &mgmt.NetworkConfig{Plugin: ct.provisioningConfig.CNIs[0]},
				Version: ct.provisioningConfig.RKE1KubernetesVersions[0],
			},
		},
	}

	clusterTemplateRevision, err := clustertemplates.CreateRkeTemplateRevision(ct.client, templateRevisonConfig, clusterTemplate.ID)
	require.NoError(ct.T(), err)

	log.Info("Create a downstream cluster as an admin.")

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(ct.client)
	require.NoError(ct.T(), err)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}

	log.Info("Verifying admin can create the cluster with the cluster template even when rke cluster template is enforced.")
	clusterObj, err := provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(ct.client, clusterTemplate.ID, clusterTemplateRevision.ID,
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

func (ct *ClusterTemplateProvisioningTestSuite) TestClusterTemplateWithQuestionsAndAnswers() {
	log.Info("Creating an rke template with questions")

	clusterTemplate, err := clustertemplates.CreateRkeTemplate(ct.client, nil)
	require.NoError(ct.T(), err)

	answers := &mgmt.Answer{Values: map[string]string{
		"rancherKubernetesEngineConfig.kubernetesVersion":                        ct.provisioningConfig.RKE1KubernetesVersions[0],
		"rancherKubernetesEngineConfig.network.plugin":                           ct.provisioningConfig.CNIs[0],
		"rancherKubernetesEngineConfig.services.etcd.backupConfig.intervalHours": "10",
	}}

	templateRevisonConfig := mgmt.ClusterTemplateRevision{
		ClusterConfig: &mgmt.ClusterSpecBase{RancherKubernetesEngineConfig: &mgmt.RancherKubernetesEngineConfig{
			Network: &mgmt.NetworkConfig{Plugin: ct.provisioningConfig.CNIs[0]},
			Version: ct.provisioningConfig.RKE1KubernetesVersions[0],
		},
		},
		Questions: Questions,
	}

	clusterTemplateRevision, err := clustertemplates.CreateRkeTemplateRevision(ct.client, templateRevisonConfig, clusterTemplate.ID)
	require.NoError(ct.T(), err)

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(ct.client)
	require.NoError(ct.T(), err)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}

	clusterObj, err := provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(ct.client, clusterTemplate.ID, clusterTemplateRevision.ID,
		nodeAndRoles, nodeTemplate, answers)
	require.NoError(ct.T(), err)

	log.Info("Verifying the rke1 cluster comes up active and the cluster object has the same answer values provided.")

	expectedHours := answers.Values["rancherKubernetesEngineConfig.services.etcd.backupConfig.intervalHours"]
	actualIntervalHoursAnswers := strconv.FormatInt(clusterObj.RancherKubernetesEngineConfig.Services.Etcd.BackupConfig.IntervalHours, 10)
	require.Equal(ct.T(), actualIntervalHoursAnswers, expectedHours)

	clusterConfig := clusters.ConvertConfigToClusterConfig(ct.provisioningConfig)
	clusterConfig.KubernetesVersion = ct.provisioningConfig.RKE1KubernetesVersions[0]
	provisioning.VerifyRKE1Cluster(ct.T(), ct.client, clusterConfig, clusterObj)
}

func (ct *ClusterTemplateProvisioningTestSuite) TestClusterTemplateEditAsAdmin() {
	log.Info("Creating an rke template with two revisions")

	clusterTemplate, err := clustertemplates.CreateRkeTemplate(ct.client, nil)
	require.NoError(ct.T(), err)

	rke1Versions, err := kubernetesversions.ListRKE1AllVersions(ct.client)
	require.NoError(ct.T(), err)
	require.True(ct.T(), len(rke1Versions) > 2)
	require.True(ct.T(), len(ct.provisioningConfig.CNIs[0]) > 1)

	templateRevisonConfig := mgmt.ClusterTemplateRevision{
		ClusterConfig: &mgmt.ClusterSpecBase{RancherKubernetesEngineConfig: &mgmt.RancherKubernetesEngineConfig{
			Network: &mgmt.NetworkConfig{Plugin: ct.provisioningConfig.CNIs[0]},
			Version: rke1Versions[len(rke1Versions)-2],
		}},
	}

	ct.provisioningConfig.RKE1KubernetesVersions[0] = templateRevisonConfig.ClusterConfig.RancherKubernetesEngineConfig.Version

	clusterTemplateRevision1, err := clustertemplates.CreateRkeTemplateRevision(ct.client, templateRevisonConfig, clusterTemplate.ID)
	require.NoError(ct.T(), err)

	log.Info("Creating rke1 template and rke template revision1")
	templateRevisonConfig.ClusterConfig.RancherKubernetesEngineConfig.Version = rke1Versions[len(rke1Versions)-1]

	log.Info("Creating a new rke template revisions in the previously created template")
	templateRevision2, err := clustertemplates.CreateRkeTemplateRevision(ct.client, templateRevisonConfig, clusterTemplate.ID)
	require.NoError(ct.T(), err)

	log.Info("Create an rke1 cluster with template revision1")

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(ct.client)
	require.NoError(ct.T(), err)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}

	clusterObj, err := provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(ct.client, clusterTemplate.ID, clusterTemplateRevision1.ID,
		nodeAndRoles, nodeTemplate, nil)
	require.NoError(ct.T(), err)

	log.Info("Verify the rke1 cluster comes up active.")

	clusterConfig := clusters.ConvertConfigToClusterConfig(ct.provisioningConfig)
	clusterConfig.KubernetesVersion = rke1Versions[len(rke1Versions)-2]
	provisioning.VerifyRKE1Cluster(ct.T(), ct.client, clusterConfig, clusterObj)

	log.Info("Update the rke1 cluster with template revision 2")
	rke1Cluster := &mgmt.Cluster{
		Name:                          namegenerator.AppendRandomString("rketemplate-cluster-"),
		ClusterTemplateID:             clusterTemplate.ID,
		ClusterTemplateRevisionID:     templateRevision2.ID,
		ClusterTemplateAnswers:        nil,
		RancherKubernetesEngineConfig: nil,
	}

	updatedClusterObj, err := extensionscluster.UpdateRKE1Cluster(ct.client, clusterObj, rke1Cluster)
	require.NoError(ct.T(), err)

	log.Info("Verify the updated rke1 cluster has new cluster revision values.")
	require.Equal(ct.T(), clusterObj.Version, updatedClusterObj.Version)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterTemplateRKE1ProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterTemplateProvisioningTestSuite))
}
