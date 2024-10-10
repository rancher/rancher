//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package observability

import (
	"context"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extencharts "github.com/rancher/shepherd/extensions/charts"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StackStateTestSuite struct {
	suite.Suite
	client           *rancher.Client
	session          *session.Session
	cluster          *management.Cluster
	extensionOptions *charts.ExtensionOptions
}

func (ss *StackStateTestSuite) TearDownSuite() {
	ss.session.Cleanup()
}

func (ss *StackStateTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ss.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(ss.T(), err)

	ss.client = client

	log.Info("Getting cluster name from the config file and append cluster details in rb")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ss.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(ss.client, clusterName)
	require.NoError(ss.T(), err, "Error getting cluster ID")
	ss.cluster, err = ss.client.Management.Cluster.ByID(clusterID)
	require.NoError(ss.T(), err)

	log.Info("Install stack state ui repository for ui extensions.")

	catalogList, err := ss.client.Catalog.ClusterRepos().List(context.TODO(), meta.ListOptions{})
	require.NoError(ss.T(), err)

	require.GreaterOrEqual(ss.T(), len(catalogList.Items), 3)

	exists := false
	for _, item := range catalogList.Items {
		if item.Name == rancherUIPlugins {
			exists = true
			break
		}
	}
	if !exists {
		_, err := ss.client.Catalog.ClusterRepos().Create(context.TODO(), &clusterRepoObj, meta.CreateOptions{})
		require.NoError(ss.T(), err)
	}

	latestSSVersion, err := ss.client.Catalog.GetLatestChartVersion(charts.StackstateChartName, charts.UIPluginName)
	require.NoError(ss.T(), err)

	ss.extensionOptions = &charts.ExtensionOptions{
		ChartName: 	charts.StackstateChartName,
		ReleaseName: charts.StackstateChartName,
		Version: latestSSVersion,
	}

}

func (ss *StackStateTestSuite) TestStackState() {

	subSession := ss.session.NewSession()
	defer subSession.Cleanup()

	client, err := ss.client.WithSession(subSession)
	require.NoError(ss.T(), err)

	ss.T().Log("Checking if the stack state extension is already installed")
	initialStackstateExtension, err := extencharts.GetChartStatus(client, "local", "cattle-ui-plugin-system", "observability")
	require.NoError(ss.T(), err)

	if !initialStackstateExtension.IsAlreadyInstalled {
		ss.T().Log("Installing stackstate ui extensions")
		err = charts.InstallStackstateExtension(client, ss.extensionOptions)
		require.NoError(ss.T(), err)
	}

	log.Info("Installing stack state CRDs")

	steveAdminClient, err := client.Steve.ProxyDownstream("local")
	require.NoError(ss.T(), err)

	crdConfig := NewStackstateCRDConfig("stackstate")

	crd, err := steveAdminClient.SteveType("observability.rancher.io.configuration").Create(crdConfig)
	require.NoError(ss.T(), err)

	appliedConfig, err := steveAdminClient.SteveType("observability.rancher.io.configuration").ByID(crd.ID)

	require.Equal(ss.T(), crd.Spec, appliedConfig.Spec)
}

func TestStackStateTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateTestSuite))
}
