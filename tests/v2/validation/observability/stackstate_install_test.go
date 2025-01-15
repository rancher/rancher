//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
	"github.com/rancher/rancher/tests/v2/actions/charts"
	"github.com/rancher/rancher/tests/v2/actions/kubeapi/namespaces"
	kubeprojects "github.com/rancher/rancher/tests/v2/actions/kubeapi/projects"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"testing"
)

type StackStateInstallTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	cluster       *clusters.ClusterMeta
	projectID     string
	catalogClient *catalog.Client
}

const (
	observabilityChartURL  = "https://charts.rancher.com/server-charts/prime/suse-observability"
	observabilityChartName = "suse-observability"
)

func (ssi *StackStateInstallTestSuite) TearDownSuite() {
	ssi.session.Cleanup()
}

func (ssi *StackStateInstallTestSuite) SetupSuite() {
	testSession := session.NewSession()
	ssi.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(ssi.T(), err)

	ssi.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ssi.T(), clusterName, "Cluster name to install should be set")
	cluster, err := clusters.NewClusterMeta(ssi.client, clusterName)
	require.NoError(ssi.T(), err)
	ssi.cluster = cluster

	ssi.catalogClient, err = ssi.client.GetClusterCatalogClient(ssi.cluster.ID)
	require.NoError(ssi.T(), err)

	//ssi.Require().NoError(ssi.pollUntilDownloaded("suse-observability", metav1.Time{}))

	projectTemplate := kubeprojects.NewProjectTemplate(cluster.ID)
	projectTemplate.Name = charts.StackstateNamespace
	project, err := client.Steve.SteveType(project).Create(projectTemplate)
	require.NoError(ssi.T(), err)
	ssi.projectID = project.ID

	ssNamespaceExists, err := namespaces.GetNamespaceByName(client, cluster.ID, charts.StackstateNamespace)
	if ssNamespaceExists == nil && k8sErrors.IsNotFound(err) {
		_, err = namespaces.CreateNamespace(client, cluster.ID, project.Name, charts.StackstateNamespace, "", map[string]string{}, map[string]string{})
	}
	require.NoError(ssi.T(), err)
}

func (ssi *StackStateInstallTestSuite) TestStackStateInstall() {
	subsession := ssi.session.NewSession()
	defer subsession.Cleanup()
	//client, err := ssi.client.WithSession(subsession)
	//require.NoError(ssi.T(), err)

	ssi.Run("Install Stackstate", func() {
		err := charts.CreateClusterRepo(ssi.client, ssi.catalogClient, observabilityChartName, observabilityChartURL)
		require.NoError(ssi.T(), err)
	})
}

func TestStackStateInstallTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateInstallTestSuite))
}
