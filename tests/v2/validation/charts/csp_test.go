//go:build (infra.any || cluster.any || sanity || validation) && !stress && !extended

package charts

import (
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/kubectl"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"
	"testing"
)

type CSPTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (c *CSPTestSuite) TearDownSuite() {
	c.session.Cleanup()
}

func (c *CSPTestSuite) SetupSuite() {
	c.session = session.NewSession()
	client, err := rancher.NewClient("", c.session)
	require.NoError(c.T(), err)
	c.client = client
}

func (c *CSPTestSuite) TestCSPAdapterMinVersionNotEmpty() {
	cmd := []string{"kubectl", "get", "setting", "csp-adapter-min-version", "-o", "yaml"}
	resp, err := kubectl.Command(c.client, nil, localCluster, cmd, "")
	require.NoError(c.T(), err)

	var data map[string]interface{}
	err = yaml.Unmarshal([]byte(resp), &data)
	require.NoError(c.T(), err)

	source, ok := data["source"].(string)
	require.True(c.T(), ok, "source field should be a string")
	require.Equal(c.T(), "env", source, "source field should be 'env'")

	value, ok := data["value"].(string)
	require.True(c.T(), ok, "value field should be a string")
	require.NotEmpty(c.T(), value, "value field should not be empty")
}

func TestCSPTestSuite(t *testing.T) {
	suite.Run(t, new(CSPTestSuite))
}
