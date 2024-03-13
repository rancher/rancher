//go:build (validation || infra.rke2k3s || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !sanity && !extended

package certrotation

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/provisioninginput"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type V2ProvCertRotationTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *provisioninginput.Config
}

func (r *V2ProvCertRotationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *V2ProvCertRotationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client
}

func (r *V2ProvCertRotationTestSuite) TestCertRotation() {
	r.Run("test-cert-rotation", func() {
		require.NoError(r.T(), rotateCerts(r.client, r.client.RancherConfig.ClusterName))
		require.NoError(r.T(), rotateCerts(r.client, r.client.RancherConfig.ClusterName))
	})
}

func TestCertRotationTestSuite(t *testing.T) {
	suite.Run(t, new(V2ProvCertRotationTestSuite))
}
