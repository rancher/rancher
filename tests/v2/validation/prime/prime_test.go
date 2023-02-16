package prime

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/rancherversion"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const rancherBrand = "suse"

type PrimeVersionTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	isPrime        bool
	rancherVersion string
}

func (t *PrimeVersionTestSuite) TearDownSuite() {
	t.session.Cleanup()
}

func (t *PrimeVersionTestSuite) SetupSuite() {
	testSession := session.NewSession(t.T())
	t.session = testSession
	primeConfig := new(rancherversion.Config)

	config.LoadConfig(rancherversion.ConfigurationFileKey, primeConfig)
	t.isPrime = primeConfig.IsPrime
	t.rancherVersion = primeConfig.RancherVersion

	client, err := rancher.NewClient("", t.session)
	assert.NoError(t.T(), err)
	t.client = client

}

func (t *PrimeVersionTestSuite) TestPrimeVersion() {
	serverConfig, err := rancherversion.RequestRancherVersion(t.client.RancherConfig.Host)
	assert.Nil(t.T(), err)

	assert.Equal(t.T(), t.isPrime, serverConfig.IsPrime)
	assert.Equal(t.T(), t.rancherVersion, serverConfig.RancherVersion)
	brand, err := t.client.Management.Setting.ByID("ui-brand")
	if err != nil {
		t.T().Log(err)
	}
	if t.isPrime {
		assert.Equal(t.T(), rancherBrand, brand.Value)
	} else {
		assert.Empty(t.T(), brand.Value)
	}
}

func TestPrimeVersionTestSuite(t *testing.T) {
	suite.Run(t, new(PrimeVersionTestSuite))
}
