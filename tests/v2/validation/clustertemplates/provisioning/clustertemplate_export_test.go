//go:build (validation || infra.rke1 || cluster.nodedriver || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke2k3s && !cluster.any && !sanity

package clustertemplates

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/clustertemplates"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	mgmt "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	log "github.com/sirupsen/logrus"
)

type ClusterTemplateExportTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
	cluster            *mgmt.Cluster
}

func (ct *ClusterTemplateExportTestSuite) TearDownSuite() {
	ct.session.Cleanup()
}

func (ct *ClusterTemplateExportTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ct.session = testSession

	ct.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, ct.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
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

	log.Info("Create a standard user.")
	user, err := users.CreateUserWithRole(ct.client, users.UserConfig(), "user")
	require.NoError(ct.T(), err)
	standardClient, err := ct.client.AsUser(user)
	require.NoError(ct.T(), err)

	log.Info("Creating an rke1 cluster")

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(standardClient)
	require.NoError(ct.T(), err)

	clusterConfig := clusters.ConvertConfigToClusterConfig(ct.provisioningConfig)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}
	clusterConfig.NodePools = nodeAndRoles

	clusterConfig.CNI = ct.provisioningConfig.CNIs[0]
	clusterConfig.KubernetesVersion = ct.provisioningConfig.RKE1KubernetesVersions[0]

	cluster, err := provisioning.CreateProvisioningRKE1Cluster(standardClient, rke1Provider, clusterConfig, nodeTemplate)
	require.NoError(ct.T(), err)

	ct.cluster, err = ct.client.Management.Cluster.ByID(cluster.ID)
	require.NoError(ct.T(), err)

	provisioning.VerifyRKE1Cluster(ct.T(), standardClient, clusterConfig, ct.cluster)
}

func (ct *ClusterTemplateExportTestSuite) TestExportClusterTemplate() {

	log.Info("Exporting the newly cluster after its provisioned as a cluster template.")
	rkeTemplateExport, err := ct.client.Management.Cluster.ActionSaveAsTemplate(ct.cluster,
		&mgmt.SaveAsTemplateInput{ClusterTemplateName: namegenerator.AppendRandomString("template"),
			ClusterTemplateRevisionName: namegenerator.AppendRandomString("revision")})
	require.NoError(ct.T(), err)

	template, err := ct.client.Management.ClusterTemplateRevision.ByID(rkeTemplateExport.ClusterTemplateRevisionName)
	require.NoError(ct.T(), err)
	require.Equal(ct.T(), template.ClusterConfig.RancherKubernetesEngineConfig.Version, ct.cluster.RancherKubernetesEngineConfig.Version)

	log.Info("Create an rke1 cluster with template revision1")

	rke1Provider := provisioning.CreateRKE1Provider(ct.provisioningConfig.Providers[0])
	nodeTemplate, err := rke1Provider.NodeTemplateFunc(ct.client)
	require.NoError(ct.T(), err)

	nodeAndRoles := []provisioninginput.NodePools{
		provisioninginput.AllRolesNodePool,
	}

	log.Info("Create a downstream cluster with the rke1 template")

	clusterObj, err := provisioning.CreateProvisioningRKE1ClusterWithClusterTemplate(ct.client, template.ID, rkeTemplateExport.ClusterTemplateRevisionName,
		nodeAndRoles, nodeTemplate, nil)
	require.NoError(ct.T(), err)

	log.Info("Verifying the rke1 cluster comes up active.")

	clusterConfig := clusters.ConvertConfigToClusterConfig(ct.provisioningConfig)
	clusterConfig.KubernetesVersion = ct.provisioningConfig.RKE1KubernetesVersions[0]
	provisioning.VerifyRKE1Cluster(ct.T(), ct.client, clusterConfig, clusterObj)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestExportClusterTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterTemplateExportTestSuite))
}
