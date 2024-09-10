package rke2

import (
	"testing"
	"time"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/reports"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/cloudcredentials"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/defaults"
	"github.com/rancher/shepherd/extensions/defaults/stevetypes"
	"github.com/rancher/shepherd/extensions/steve"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	localCluster          = "local"
	fleetNamespace        = "fleet-default"
	providerName          = "rke2"
	templateTestConfigKey = "templateTest"
)

type ClusterTemplateTestSuite struct {
	suite.Suite
	client             *rancher.Client
	standardUserClient *rancher.Client
	session            *session.Session
	templateConfig     *provisioninginput.TemplateConfig
	cloudCredentials   *v1.SteveAPIObject
}

func (r *ClusterTemplateTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *ClusterTemplateTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.templateConfig = new(provisioninginput.TemplateConfig)
	config.LoadConfig(templateTestConfigKey, r.templateConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)
	r.client = client

	provider := provisioning.CreateProvider(r.templateConfig.TemplateProvider)
	cloudCredentialConfig := cloudcredentials.LoadCloudCredential(r.templateConfig.TemplateProvider)
	r.cloudCredentials, err = provider.CloudCredFunc(client, cloudCredentialConfig)
	require.NoError(r.T(), err)
}

func (r *ClusterTemplateTestSuite) TestProvisionRKE2TemplateCluster() {
	_, err := steve.CreateAndWaitForResource(r.client, stevetypes.ClusterRepo, r.templateConfig.Repo, true, 5*time.Second, defaults.FiveMinuteTimeout)
	require.NoError(r.T(), err)

	k8sversions, err := kubernetesversions.Default(r.client, providerName, nil)
	require.NoError(r.T(), err)

	clusterName := namegenerator.AppendRandomString(providerName + "-template")
	err = charts.InstallTemplateChart(r.client, r.templateConfig.Repo.ObjectMeta.Name, r.templateConfig.TemplateName, clusterName, k8sversions[0], r.cloudCredentials)
	require.NoError(r.T(), err)

	_, cluster, err := clusters.GetProvisioningClusterByName(r.client, clusterName, fleetNamespace)
	reports.TimeoutClusterReport(cluster, err)
	require.NoError(r.T(), err)

	provisioning.VerifyCluster(r.T(), r.client, nil, cluster)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestClusterTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(ClusterTemplateTestSuite))
}
