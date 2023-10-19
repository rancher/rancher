package configmaps

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SteveAPITestSuite struct {
  suite.Suite
  client            *rancher.Client
  session           *session.Session
  cluster           *management.Cluster
}

func (s *SteveAPITestSuite) TearDownSuite() {
  s.session.Cleanup()
}

func (s *SteveAPITestSuite) SetupSuite() {
  testSession := session.NewSession()
  s.session = testSession

  client, err := rancher.NewClient("", testSession)
  require.NoError(s.T(), err)
  s.client = client
}



func (s *SteveAPITestSuite) TestConfigMapCreate() {
  subSession := s.session.NewSession()
  defer subSession.Cleanup()

  client, err := rancher.NewClient("", subSession)
  require.NoError(s.T(), err)

  s.client = client

  log.Info("Getting cluster name from the config file and append cluster details in s")
  clusterName := client.RancherConfig.ClusterName
  require.NotEmptyf(s.T(), clusterName, "Cluster name to install should be set")
  clusterID, err := clusters.GetClusterIDByName(s.client, clusterName)
  require.NoError(s.T(), err, "Error getting cluster ID")
  s.cluster, err = s.client.Management.Cluster.ByID(clusterID)
  require.NoError(s.T(), err)

  s.Run("Configmap Create Operation.", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    configmapNamespace := namespace
    configmapObj := createConfigmap(*configMapClient, configmapName, configmapNamespace, annotations, labels, data)
    require.NoError(s.T(), err)

    // Check that the configmap has been created
    configmapByID, err := configMapClient.ByID(configmapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), configmapByID.JSONResp["id"], configmapName)
  })
}

func (s *SteveAPITestSuite) TestConfigMapUpdate(){
  subSession := s.session.NewSession()
  defer subSession.Cleanup()

  client, err := rancher.NewClient("", subSession)
  require.NoError(s.T(), err)

  s.client = client

  log.Info("Getting cluster name from the config file and append cluster details in s")
  clusterName := client.RancherConfig.ClusterName
  require.NotEmptyf(s.T(), clusterName, "Cluster name to install should be set")
  clusterID, err := clusters.GetClusterIDByName(s.client, clusterName)
  require.NoError(s.T(), err, "Error getting cluster ID")
  s.cluster, err = s.client.Management.Cluster.ByID(clusterID)
  require.NoError(s.T(), err)

  s.Run("Configmap Update Operation.", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    configmapNamespace := namespace
    configmapObj := createConfigmap(*configMapClient, configmapName, configmapNamespace, annotations, labels, data)
    require.NoError(s.T(), err)

    // Update action
    updatedConfigMapObj, err := updateConfigmapAnnotations(*configMapClient, configmapObj, updatedAnnotations)
    require.NoError(s.T(), err)

    // Check for updated annotation
    for i := 0; i < len(updatedConfigMapObj.ObjectMeta.Annotations); i++ {
      assert.Contains(s.T(), updatedConfigMapObj.ObjectMeta.Annotations, "anno2")
      assert.Contains(s.T(), updatedConfigMapObj.ObjectMeta.Annotations["anno2"], "new automated annotation")
      assert.Contains(s.T(), updatedConfigMapObj.ObjectMeta.Annotations, "field.cattle.io/description")
      assert.Contains(s.T(), updatedConfigMapObj.ObjectMeta.Annotations["field.cattle.io/description"], "new automated configmap description")
    }
  })
}

func (s *SteveAPITestSuite) TestConfigmapDelete(){
  subSession := s.session.NewSession()
  s.session = subSession
  defer subSession.Cleanup()

  client, err := rancher.NewClient("", subSession)
  require.NoError(s.T(), err)

  s.client = client

  log.Info("Getting cluster name from the config file and append cluster details in s")
  clusterName := client.RancherConfig.ClusterName
  require.NotEmptyf(s.T(), clusterName, "Cluster name to install should be set")
  clusterID, err := clusters.GetClusterIDByName(s.client, clusterName)
  require.NoError(s.T(), err, "Error getting cluster ID")
  s.cluster, err = s.client.Management.Cluster.ByID(clusterID)
  require.NoError(s.T(), err)

  s.Run("Configmap Delete Operation.", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    configmapNamespace := namespace
    configmapObj := createConfigmap(*configMapClient, configmapName, configmapNamespace, annotations, labels, data)
    require.NoError(s.T(), err)

    //delete
    err = deleteConfigmap(*configMapClient, configmapObj)
    require.NoError(s.T(), err)

    // validate deletion
    configmapByID, err := configMapClient.ByID(configmapObj.ID)
    require.Error(s.T(), err)
    assert.Nil(s.T(), configmapByID)
  })
}

func TestConfigMapsSuite(t *testing.T) {
  suite.Run(t, new(SteveAPITestSuite))
}
