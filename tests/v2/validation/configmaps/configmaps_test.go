package configmaps

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/namegenerator"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

var(
  configmapName = namegenerator.AppendRandomString(("steve-configmap"))
  updatedConfigmapName= namegenerator.AppendRandomString(("updated-configmap"))
)

func (s *SteveAPITestSuite) TestConfigMapCreate() {
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

  s.Run("Configmap Create Operation.", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    const configmapNamespace = "default"

    configMapObj, err := configMapClient.Create(corev1.ConfigMap{
      ObjectMeta: metav1.ObjectMeta{
        Name:        configmapName,
        Namespace:   configmapNamespace,
        Annotations: map[string]string{"anno1": "automated annotation", "field.cattle.io/description": "automated configmap description"},
        Labels:      map[string]string{"label1": "autoLabel"},
      },
      Data: map[string]string{"foo": "bar"},
    })
    require.NoError(s.T(), err)

    // Check that the configmap has been created
    configmapByID, err := configMapClient.ByID(configMapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), configmapByID.JSONResp["id"], configmapName)
  })
}

func (s *SteveAPITestSuite) TestConfigMapUpdate(){
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

  s.Run("Configmap Update Operation.", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    const configmapNamespace = "default"

    configMapObj, err := configMapClient.Create(corev1.ConfigMap{
      ObjectMeta: metav1.ObjectMeta{
        Name:        configmapName,
        Namespace:   configmapNamespace,
        Annotations: map[string]string{"anno1": "automated annotation", "field.cattle.io/description": "automated configmap description"},
        Labels:      map[string]string{"label1": "autoLabel"},
      },
      Data: map[string]string{"foo": "bar"},
    })
    require.NoError(s.T(), err)

    // Check that the configmap has been created
    configmapByID, err := configMapClient.ByID(configMapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), configmapByID.JSONResp["id"], configmapName)
    
    // Update action
    configMapObj, err = configMapClient.Update(configmapByID, configmapByID.JSONResp["id"]{})
    require.NoError(s.T(), err)

    // Check for updated name
    configmapByID, err = configMapClient.ByID(configMapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), configmapByID.JSONResp["id"], updatedConfigmapName)
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

  s.Run("Configmap Update Operation.", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    const configmapNamespace = "default"

    configMapObj, err := configMapClient.Create(corev1.ConfigMap{
      ObjectMeta: metav1.ObjectMeta{
        Name:        configmapName,
        Namespace:   configmapNamespace,
        Annotations: map[string]string{"anno1": "automated annotation", "field.cattle.io/description": "automated configmap description"},
        Labels:      map[string]string{"label1": "autoLabel"},
      },
      Data: map[string]string{"foo": "bar"},
    })
    require.NoError(s.T(), err)

    // Check that the configmap has been created
    configmapByID, err := configMapClient.ByID(configMapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), configmapByID.JSONResp["id"], configmapName)

    // delete
    err = configMapClient.Delete(configmapByID)
    require.NoError(s.T(), err)

    // validate deletion
    configmapByID, err = configMapClient.ByID(configMapObj.ID)
    require.Error(s.T(), err)
    assert.Nil(s.T(), configmapByID)
  })
}


func TestConfigMapsSuite(t *testing.T) {
  suite.Run(t, new(SteveAPITestSuite))
}
