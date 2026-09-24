package integration

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/suite"
)

type ClusterDefaultsTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (s *ClusterDefaultsTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	client, err := rancher.NewClient("", testSession)
	s.Require().NoError(err)
	s.client = client
}

func (s *ClusterDefaultsTestSuite) TearDownSuite() {
	s.session.Cleanup()
}

// TestImportInitialConditions asserts that a newly created import cluster
// has no conditions set immediately after creation, mirroring the Python
// test test_import_initial_conditions in test_cluster_defaults.py.
func (s *ClusterDefaultsTestSuite) TestImportInitialConditions() {
	subSession := s.session.NewSession()
	defer subSession.Cleanup()

	client, err := s.client.WithSession(subSession)
	s.Require().NoError(err)

	cluster, err := client.Management.Cluster.Create(&management.Cluster{
		Name: namegen.AppendRandomString("cluster-"),
	})
	s.Require().NoError(err)

	s.Empty(cluster.Conditions, "expected no conditions on a newly created import cluster")
}

func TestClusterDefaults(t *testing.T) {
	suite.Run(t, new(ClusterDefaultsTestSuite))
}
