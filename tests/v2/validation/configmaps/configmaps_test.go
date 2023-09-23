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
  userClients       map[string]*rancher.Client
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

  s.userClients = make(map[string]*rancher.Client)
}

func (s *SteveAPITestSuite) TestConfigMapCrud() {
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

  s.Run("global", func() {
    configMapClient := s.client.Steve.SteveType("configmap")

    // create
    configMapObj, err := configMapClient.Create(corev1.ConfigMap{
      ObjectMeta: metav1.ObjectMeta{
        Name:        namegenerator.AppendRandomString("steve-configmap"),
        Namespace:   "default", // need to specify the namespace for a namespaced resource if using a global endpoint ("/v1/configmaps")
        Annotations: map[string]string{"anno1": "automated annotation", "field.cattle.io/description": "automated configmap description"},
        Labels:      map[string]string{"label1": "autoLabel"},
      },
      Data: map[string]string{"foo": "bar"},
    })
    require.NoError(s.T(), err)

    // read
    readObj, err := configMapClient.ByID(configMapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), readObj.JSONResp["data"], "foo", "bar")
    assert.Contains(s.T(), readObj.JSONResp["metadata"], "annotations", "anno1", "automated annotation")
    assert.Contains(s.T(), readObj.JSONResp["metadata"], "annotations", "field.cattle.io/description", "automated configmap description")
    assert.Contains(s.T(), readObj.JSONResp["metadata"], "labels", "label1", "autolabel")

    // update
    updatedConfigMap := configMapObj.JSONResp
    updatedConfigMap["data"] = map[string]string{"lorem": "ipsum"}
    updatedConfigMap["annotations"] = map[string]string{"anno2": "updated auto annotation", "field.cattle.io/description": "updated auto configmap description"}
    updatedConfigMap["labels"] = map[string]string{"label2": "updated label"}

    configMapObj, err = configMapClient.Update(configMapObj, &updatedConfigMap)
    require.NoError(s.T(), err)

    // read again
    readObj, err = configMapClient.ByID(configMapObj.ID)
    require.NoError(s.T(), err)
    assert.Contains(s.T(), readObj.JSONResp["data"], "lorem", "ipsum")
    assert.Contains(s.T(), readObj.JSONResp["metadata"], "annotations", "anno1", "automated annotation")
    assert.Contains(s.T(), readObj.JSONResp["metadata"], "annotations", "field.cattle.io/description", "automated configmap description")
    assert.Contains(s.T(), readObj.JSONResp["metadata"], "labels", "label2", "updated label")

    // delete
    err = configMapClient.Delete(readObj)
    require.NoError(s.T(), err)

    // read again
    readObj, err = configMapClient.ByID(configMapObj.ID)
    require.Error(s.T(), err)
    assert.Nil(s.T(), readObj)
  })
}

func TestConfigMapsSuite(t *testing.T) {
  suite.Run(t, new(SteveAPITestSuite))
}
