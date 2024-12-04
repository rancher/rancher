//go:build validation || sanity

package fleet

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	extensionscluster "github.com/rancher/shepherd/extensions/clusters"
	extensionsfleet "github.com/rancher/shepherd/extensions/fleet"
	"github.com/rancher/shepherd/extensions/workloads/pods"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FleetPublicRepoTestSuite struct {
	suite.Suite
	client    *rancher.Client
	session   *session.Session
	clusterID string
}

func (f *FleetPublicRepoTestSuite) TearDownSuite() {
	f.session.Cleanup()
}

func (f *FleetPublicRepoTestSuite) SetupSuite() {
	f.session = session.NewSession()

	client, err := rancher.NewClient("", f.session)
	require.NoError(f.T(), err)

	f.client = client

	userConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, userConfig)

	clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(f.client, f.client.RancherConfig.ClusterName, fleet.Namespace)
	if clusterObject != nil {
		status := &provv1.ClusterStatus{}
		err := steveV1.ConvertToK8sType(clusterObject.Status, status)
		require.NoError(f.T(), err)

		f.clusterID = status.ClusterName
	} else {
		f.clusterID, err = extensionscluster.GetClusterIDByName(f.client, f.client.RancherConfig.ClusterName)
		require.NoError(f.T(), err)
	}

	podErrors := pods.StatusPods(f.client, f.clusterID)
	require.Empty(f.T(), podErrors)
}

func (f *FleetPublicRepoTestSuite) TestGitRepoDeployment() {
	defer f.session.Cleanup()

	fleetVersion, err := fleet.GetDeploymentVersion(f.client, fleet.FleetControllerName, fleet.LocalName)
	require.NoError(f.T(), err)

	urlQuery, err := url.ParseQuery(fmt.Sprintf("labelSelector=%s=%s", "cattle.io/os", "windows"))
	require.NoError(f.T(), err)

	steveClient, err := f.client.Steve.ProxyDownstream(f.clusterID)
	require.NoError(f.T(), err)

	winsNodeList, err := steveClient.SteveType("node").List(urlQuery)
	require.NoError(f.T(), err)

	if len(winsNodeList.Data) > 0 {
		fleetVersion += " windows"
	}

	f.Run("fleet "+fleetVersion, func() {
		fleetGitRepo := v1alpha1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fleet.FleetMetaName + namegenerator.RandStringLower(5),
				Namespace: fleet.Namespace,
			},
			Spec: v1alpha1.GitRepoSpec{
				Repo:            fleet.ExampleRepo,
				Branch:          fleet.BranchName,
				Paths:           []string{fleet.GitRepoPathLinux},
				CorrectDrift:    &v1alpha1.CorrectDrift{},
				ImageScanCommit: v1alpha1.CommitSpec{AuthorName: "", AuthorEmail: ""},
				Targets:         []v1alpha1.GitTarget{{ClusterName: f.client.RancherConfig.ClusterName}},
			},
		}

		if len(winsNodeList.Data) > 0 {
			fleetGitRepo.Spec.Paths = []string{fleet.GitRepoPathWindows}
		}

		f.client, err = f.client.ReLogin()
		require.NoError(f.T(), err)

		logrus.Info("Deploying public fleet gitRepo")
		gitRepoObject, err := extensionsfleet.CreateFleetGitRepo(f.client, &fleetGitRepo)
		require.NoError(f.T(), err)

		err = fleet.VerifyGitRepo(f.client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+f.client.RancherConfig.ClusterName)
		require.NoError(f.T(), err)

	})
}

func (f *FleetPublicRepoTestSuite) TestDynamicGitRepoDeployment() {

	testSession := session.NewSession()
	defer testSession.Cleanup()
	client, err := f.client.WithSession(testSession)
	require.NoError(f.T(), err)

	dynamicGitRepo := fleet.GitRepoConfig()
	require.NotNil(f.T(), dynamicGitRepo)

	if len(dynamicGitRepo.Spec.Targets) < 1 {
		dynamicGitRepo.Spec.Targets = []v1alpha1.GitTarget{
			{
				ClusterName: client.RancherConfig.ClusterName,
			},
		}
	}

	fleetVersion, err := fleet.GetDeploymentVersion(client, fleet.FleetControllerName, fleet.LocalName)
	require.NoError(f.T(), err)

	f.Run("fleet "+fleetVersion, func() {
		client, err = client.ReLogin()
		require.NoError(f.T(), err)

		logrus.Info("Deploying dynamic gitRepo: ", dynamicGitRepo.Spec)

		gitRepoObject, err := extensionsfleet.CreateFleetGitRepo(client, dynamicGitRepo)
		require.NoError(f.T(), err)

		// expects dynamicGitRepo.GitRepoSpec.Targets to include RancherConfig.ClusterName
		err = fleet.VerifyGitRepo(client, gitRepoObject.ID, f.clusterID, fleet.Namespace+"/"+client.RancherConfig.ClusterName)
		require.NoError(f.T(), err)
	})
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFleetPublicRepoTestSuite(t *testing.T) {
	suite.Run(t, new(FleetPublicRepoTestSuite))
}
