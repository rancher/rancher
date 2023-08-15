package certrotation

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/provisioninginput"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
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
		require.NoError(r.T(), RotateCerts(r.client, r.client.RancherConfig.ClusterName))
		require.NoError(r.T(), RotateCerts(r.client, r.client.RancherConfig.ClusterName))
	})
}

func TestCertRotation(t *testing.T) {
	suite.Run(t, new(V2ProvCertRotationTestSuite))
}
