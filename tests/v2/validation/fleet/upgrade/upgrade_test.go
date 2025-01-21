package update

import (
	"net/url"
	"testing"

	"errors"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/services"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	extensionClusters "github.com/rancher/shepherd/extensions/clusters"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nodePoolsize   = 3
	guestbookLabel = "labelSelector=app=guestbook"
)

type UpgradeTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (u *UpgradeTestSuite) TearDownSuite() {
	u.session.Cleanup()
}

func (u *UpgradeTestSuite) SetupSuite() {
	u.session = session.NewSession()

	client, err := rancher.NewClient("", u.session)
	require.NoError(u.T(), err)

	u.client = client

	log.Info("Getting cluster name from the config file and append cluster details in connection")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(u.T(), clusterName, "Cluster name to install should be set")

	clusterID, err := extensionClusters.GetClusterIDByName(u.client, clusterName)
	require.NoError(u.T(), err, "Error getting cluster ID")

	u.cluster, err = u.client.Management.Cluster.ByID(clusterID)
	require.NoError(u.T(), err)
}

func (u *UpgradeTestSuite) TestDeployFleetRepoUpgrade() {

	steveClient, err := u.client.Steve.ProxyDownstream(u.cluster.ID)
	require.NoError(u.T(), err)

	err = clusters.VerifyNodePoolSize(steveClient, nodePoolsize)
	if errors.Is(err, clusters.SmallerPoolClusterSize) {
		u.T().Skip("The deploy fleet repo and upgrade test requires at least 3 worker nodes.")
	} else {
		require.NoError(u.T(), err)
	}

	u.T().Log("Creating a fleet git repo")
	fleetGitRepo := &v1alpha1.GitRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
			Namespace: fleet.Namespace,
		},
		Spec: v1alpha1.GitRepoSpec{
			Repo:   fleet.ExampleRepo,
			Branch: fleet.BranchName,
			Paths:  []string{fleet.GitRepoPathLinux},
			Targets: []v1alpha1.GitTarget{
				{
					ClusterName: u.cluster.Name,
				},
			},
		},
	}

	repoObject, err := extensionsfleet.CreateFleetGitRepo(u.client, fleetGitRepo)
	require.NoError(u.T(), err)

	err = fleet.VerifyGitRepo(u.client, repoObject.ID, u.cluster.ID, fleet.Namespace+"/"+u.cluster.Name)
	require.NoError(u.T(), err)

	query, err := url.ParseQuery(guestbookLabel)
	require.NoError(u.T(), err)

	servicesResp, err := steveClient.SteveType(services.ServiceSteveType).List(query)
	require.NoError(u.T(), err)
	require.NotEmpty(u.T(), servicesResp.Data)

	for _, serviceResp := range servicesResp.Data {
		err = services.VerifyClusterIP(u.client, u.cluster.Name, steveClient, serviceResp.ID, "80", "Guestbook")
		require.NoError(u.T(), err)
	}

	err = u.client.Steve.SteveType("fleet.cattle.io.gitrepo").Delete(repoObject)
	require.NoError(u.T(), err)
}

func TestUpgradeTestSuite(t *testing.T) {
	suite.Run(t, new(UpgradeTestSuite))
}
