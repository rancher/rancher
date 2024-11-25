//go:build validation

package harvester

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/harvester"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/sirupsen/logrus"

	harvesteraction "github.com/rancher/rancher/tests/v2/actions/harvester"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type HarvesterTestSuite struct {
	suite.Suite
	client          *rancher.Client
	session         *session.Session
	clusterID       string
	harvesterClient *harvester.Client
}

func (h *HarvesterTestSuite) TearDownSuite() {
	h.session.Cleanup()
}

func (h *HarvesterTestSuite) SetupSuite() {
	h.session = session.NewSession()

	client, err := rancher.NewClient("", h.session)
	require.NoError(h.T(), err)

	h.client = client

	hClient, err := harvester.NewClient("", h.session)
	require.NoError(h.T(), err)

	h.harvesterClient = hClient

	userConfig := new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, userConfig)
}

func (h *HarvesterTestSuite) TestImport() {
	harvesterInRancherID, err := harvesteraction.RegisterHarvesterWithRancher(h.client, h.harvesterClient)
	require.NoError(h.T(), err)
	logrus.Info(harvesterInRancherID)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestHarvesterTestSuite(t *testing.T) {
	suite.Run(t, new(HarvesterTestSuite))
}
