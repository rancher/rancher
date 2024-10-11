//go:build validation

package k3s

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KdmChecksTestSuite struct {
	suite.Suite
	session            *session.Session
	client             *rancher.Client
	ns                 string
	provisioningConfig *provisioninginput.Config
}

const (
	defaultNamespace = "default"
)

func (k *KdmChecksTestSuite) TearDownSuite() {
	k.session.Cleanup()
}

func (k *KdmChecksTestSuite) SetupSuite() {
	testSession := session.NewSession()
	k.session = testSession

	k.ns = defaultNamespace

	k.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, k.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(k.T(), err)

	k.client = client
}

func (k *KdmChecksTestSuite) TestK3SK8sVersions() {
	logrus.Infof("checking for valid k8s versions..")
	require.GreaterOrEqual(k.T(), len(k.provisioningConfig.K3SKubernetesVersions), 1)
	// fetching all available k8s versions from rancher
	releasedK8sVersions, _ := kubernetesversions.ListK3SAllVersions(k.client)
	logrus.Info("expected k8s versions : ", k.provisioningConfig.K3SKubernetesVersions)
	logrus.Info("k8s versions available on rancher server : ", releasedK8sVersions)
	for _, expectedK8sVersion := range k.provisioningConfig.K3SKubernetesVersions {
		require.Contains(k.T(), releasedK8sVersions, expectedK8sVersion)
	}
}

func (k *KdmChecksTestSuite) TestProvisioningSingleNodeK3SClusters() {
	require.GreaterOrEqual(k.T(), len(k.provisioningConfig.Providers), 1)
	permutations.RunTestPermutations(&k.Suite, "oobRelease-", k.client, k.provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
}

func TestPostKdmOutOfBandReleaseChecks(t *testing.T) {
	suite.Run(t, new(KdmChecksTestSuite))
}
