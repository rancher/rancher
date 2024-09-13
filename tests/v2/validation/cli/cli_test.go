//go:build validation

package cli

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/cli"
	"github.com/rancher/shepherd/clients/rancher"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type CLITestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (c *CLITestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CLITestSuite) SetupSuite() {
	testSession := session.NewSession()
	c.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(c.T(), err)

	c.client = client
}

func (c *CLITestSuite) TestContext() {
	err := cli.SwitchContext(c.client.CLI)
	require.NoError(c.T(), err)
}

func (c *CLITestSuite) TestProjects() {
	var projectName = namegen.AppendRandomString("projects")
	var clusterName = namegen.AppendRandomString("cluster")

	err := cli.CreateProjects(c.client.CLI, projectName, "local")
	require.NoError(c.T(), err)

	err = cli.CreateProjects(c.client.CLI, projectName, clusterName)
	require.Error(c.T(), err)

	err = cli.DeleteProjects(c.client.CLI, projectName)
	require.NoError(c.T(), err)
}

func (c *CLITestSuite) TestNamespaces() {
	var namespaceName = namegen.AppendRandomString("ns")
	var projectName = namegen.AppendRandomString("projects")

	err := cli.CreateNamespaces(c.client.CLI, namespaceName, projectName)
	require.NoError(c.T(), err)

	err = cli.DeleteNamespaces(c.client.CLI, namespaceName, projectName)
	require.NoError(c.T(), err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestCLITestSuite(t *testing.T) {
	suite.Run(t, new(CLITestSuite))
}
