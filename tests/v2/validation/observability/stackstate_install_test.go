//go:build (validation || infra.any || cluster.k3s || sanity) && !stress && !extended

package observability

import (
	"context"
	rv1 "github.com/rancher/rancher/pkg/apis/catalog.cattle.io/v1"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kwait "k8s.io/apimachinery/pkg/util/wait"
	"testing"
	"time"
)

type StackStateInstallTestSuite struct {
	suite.Suite
	client        *rancher.Client
	session       *session.Session
	cluster       *clusters.ClusterMeta
	projectID     string
	catalogClient *catalog.Client
}

var (
	PollInterval = time.Duration(500 * time.Millisecond)
	propagation  = metav1.DeletePropagationForeground
)

func (ssi *StackStateInstallTestSuite) TearDownSuite() {
	ssi.Require().NoError(ssi.catalogClient.ClusterRepos().Delete(context.Background(), "suse-observability", metav1.DeleteOptions{PropagationPolicy: &propagation}))
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

	_, err = ssi.catalogClient.ClusterRepos().Create(context.Background(), &rv1.ClusterRepo{
		ObjectMeta: metav1.ObjectMeta{Name: "suse-observability"},
		Spec:       rv1.RepoSpec{URL: "https://charts.rancher.com/server-charts/prime/suse-observability"}}, metav1.CreateOptions{})
	ssi.Require().NoError(err)
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

		err := charts.InstallStackStateWithHelm()
		require.NoError(ssi.T(), err)
	})

}

// pollUntilDownloaded Polls until the ClusterRepo of the given name has been downloaded (by comparing prevDownloadTime against the current DownloadTime)
func (ssi *StackStateInstallTestSuite) pollUntilDownloaded(ClusterRepoName string, prevDownloadTime metav1.Time) error {
	err := kwait.Poll(PollInterval, time.Minute, func() (done bool, err error) {
		clusterRepo, err := ssi.catalogClient.ClusterRepos().Get(context.TODO(), ClusterRepoName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		ssi.Require().NoError(err)

		return clusterRepo.Status.DownloadTime != prevDownloadTime, nil
	})
	return err
}

func TestStackStateInstallTestSuite(t *testing.T) {
	suite.Run(t, new(StackStateInstallTestSuite))
}
