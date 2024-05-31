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

type FleetTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (f *FleetTestSuite) TearDownSuite() {
	f.session.Cleanup()
}

func (f *FleetTestSuite) SetupSuite() {
	f.session = session.NewSession()
	client, err := rancher.NewClient("", f.session)
	require.NoError(f.T(), err)
	f.client = client
}

func (f *FleetTestSuite) TestFleetVersionNotEmpty() {
	cmd := []string{"kubectl", "get", "setting", "fleet-version", "-o", "yaml"}
	resp, err := kubectl.Command(f.client, nil, localCluster, cmd, "")
	require.NoError(f.T(), err)

	var data map[string]interface{}
	err = yaml.Unmarshal([]byte(resp), &data)
	require.NoError(f.T(), err)

	source, ok := data["source"].(string)
	require.True(f.T(), ok, "source field should be a string")
	require.Equal(f.T(), "env", source, "source field should be 'env'")

	value, ok := data["value"].(string)
	require.True(f.T(), ok, "value field should be a string")
	require.NotEmpty(f.T(), value, "value field should not be empty")
}

func TestFleetTestSuite(t *testing.T) {
	suite.Run(t, new(FleetTestSuite))
}
