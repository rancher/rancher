//go:build validation || airgap

package fleet

import (
	"testing"

	"github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/fleet"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/clients/rancher/catalog"
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
	client             *rancher.Client
	session            *session.Session
	provisioningConfig *provisioninginput.Config
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

	f.provisioningConfig = userConfig
}

func (f *FleetPublicRepoTestSuite) TestGitRepoDeployment() {
	defer f.session.Cleanup()

	var clusterID string
	var err error
	clusterObject, _, _ := extensionscluster.GetProvisioningClusterByName(f.client, f.client.RancherConfig.ClusterName, "fleet-default")
	if clusterObject != nil {
		status := &provv1.ClusterStatus{}
		err := steveV1.ConvertToK8sType(clusterObject.Status, status)
		require.NoError(f.T(), err)

		clusterID = status.ClusterName
		logrus.Info(clusterID)
	} else {
		clusterID, err = extensionscluster.GetClusterIDByName(f.client, f.client.RancherConfig.ClusterName)
		require.NoError(f.T(), err)
	}

	podErrors := pods.StatusPods(f.client, clusterID)
	require.Empty(f.T(), podErrors)

	fleetVersion, err := fleet.GetDeploymentVersion(f.client, fleet.FleetControllerName, "local")
	require.NoError(f.T(), err)

	chartVersion, err := f.client.Catalog.GetLatestChartVersion("fleet", catalog.RancherChartRepo)
	require.NoError(f.T(), err)
	require.Contains(f.T(), chartVersion, fleetVersion[1:])

	f.Run("/fleet "+fleetVersion, func() {
		fleetGitRepo := v1alpha1.GitRepo{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "automatedrepo-" + namegenerator.RandStringLower(5),
				Namespace: "fleet-default",
			},
			Spec: v1alpha1.GitRepoSpec{
				Repo:                "https://github.com/rancher/fleet-examples", // "https://github.com/rancher/fleet-examplestypo",
				Branch:              "master",
				Paths:               []string{"simple"},
				CorrectDrift:        &v1alpha1.CorrectDrift{},
				ImageScanCommit:     v1alpha1.CommitSpec{AuthorName: "", AuthorEmail: ""},
				ForceSyncGeneration: 1,
				Targets:             []v1alpha1.GitTarget{{ClusterName: f.client.RancherConfig.ClusterName}},
			},
		}

		f.client, err = f.client.ReLogin()
		require.NoError(f.T(), err)

		logrus.Info("Deploying public fleet gitRepo")
		gitRepoObject, err := extensionsfleet.CreateFleetGitRepo(f.client, &fleetGitRepo)
		require.NoError(f.T(), err)

		fleet.VerifyGitRepo(f.T(), f.client, gitRepoObject.ID, clusterID, "fleet-default/"+f.client.RancherConfig.ClusterName)

	})
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFleetPublicRepoTestSuite(t *testing.T) {
	suite.Run(t, new(FleetPublicRepoTestSuite))
}
