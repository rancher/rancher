//go:build validation

package prime

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	prime "github.com/rancher/rancher/tests/framework/extensions/prime"
	"github.com/rancher/rancher/tests/framework/extensions/rancherversion"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	systemRegistry = "system-default-registry"
	localCluster   = "local"
	uiBrand        = "ui-brand"
)

type PrimeTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	brand          string
	isPrime        bool
	rancherVersion string
	primeRegistry  string
}

func (t *PrimeTestSuite) TearDownSuite() {
	t.session.Cleanup()
}

func (t *PrimeTestSuite) SetupSuite() {
	testSession := session.NewSession()
	t.session = testSession

	primeConfig := new(rancherversion.Config)
	config.LoadConfig(rancherversion.ConfigurationFileKey, primeConfig)

	t.brand = primeConfig.Brand
	t.isPrime = primeConfig.IsPrime
	t.rancherVersion = primeConfig.RancherVersion
	t.primeRegistry = primeConfig.Registry

	client, err := rancher.NewClient("", t.session)
	assert.NoError(t.T(), err)

	t.client = client
}

func (t *PrimeTestSuite) TestPrimeUIBrand() {
	rancherBrand, err := t.client.Management.Setting.ByID(uiBrand)
	require.NoError(t.T(), err)

	checkBrand := prime.CheckUIBrand(t.client, t.isPrime, rancherBrand, t.brand)
	assert.NoError(t.T(), checkBrand)
}

func (t *PrimeTestSuite) TestPrimeVersion() {
	serverConfig, err := rancherversion.RequestRancherVersion(t.client.RancherConfig.Host)
	require.NoError(t.T(), err)

	checkVersion := prime.CheckVersion(t.isPrime, t.rancherVersion, serverConfig)
	assert.NoError(t.T(), checkVersion)
}

func (t *PrimeTestSuite) TestSystemDefaultRegistry() {
	registry, err := t.client.Management.Setting.ByID(systemRegistry)
	require.NoError(t.T(), err)

	checkRegistry := prime.CheckSystemDefaultRegistry(t.isPrime, t.primeRegistry, registry)
	assert.NoError(t.T(), checkRegistry)
}

func (t *PrimeTestSuite) TestLocalClusterRancherImages() {
	podResults, podErrors := pods.StatusPods(t.client, localCluster)
	assert.Empty(t.T(), podErrors)
	assert.NotEmpty(t.T(), podResults)
}

func TestPrimeTestSuite(t *testing.T) {
	suite.Run(t, new(PrimeTestSuite))
}
