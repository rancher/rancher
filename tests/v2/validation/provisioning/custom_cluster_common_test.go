package provisioning

import (
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/users"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CustomClusterProvisioningTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	kubernetesVersions []string
	hasWindows         bool
	cnis               []string
	nodeProviders      []string
}

func (c *CustomClusterProvisioningTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CustomClusterProvisioningTestSuite) SetupSuiteLinuxOnly() {
	testSession := session.NewSession(c.T())
	c.session = testSession

	clustersConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, clustersConfig)

	c.kubernetesVersions = clustersConfig.KubernetesVersions
	c.cnis = clustersConfig.CNIs
	c.nodeProviders = clustersConfig.NodeProviders
	c.hasWindows = false

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	enabled := true
	var testuser = AppendRandomString("testuser-")
	user := &management.User{
		Username: testuser,
		Password: "rancherrancher123!",
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(c.T(), err)
	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(c.T(), err)

	c.standardUserClient = standardUserClient
}

func (c *CustomClusterProvisioningTestSuite) SetupSuiteHybrid() {
	testSession := session.NewSession(c.T())
	c.session = testSession

	clustersConfig := new(Config)
	config.LoadConfig(ConfigurationFileKey, clustersConfig)

	c.kubernetesVersions = clustersConfig.KubernetesVersions
	c.cnis = clustersConfig.CNIs
	c.nodeProviders = clustersConfig.NodeProviders
	c.hasWindows = true

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client

	enabled := true
	var testuser = AppendRandomString("testuser-")
	user := &management.User{
		Username: testuser,
		Password: "rancherrancher123!",
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(c.T(), err)
	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(c.T(), err)

	c.standardUserClient = standardUserClient
}
