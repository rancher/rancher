package configmaps

import (
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/configmaps"
	"github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigmapTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (cm *ConfigmapTestSuite) TearDownSuite() {
	cm.session.Cleanup()
}

func (cm *ConfigmapTestSuite) SetupSuite() {
	cm.session = session.NewSession()

	client, err := rancher.NewClient("", cm.session)
	require.NoError(cm.T(), err)
	cm.client = client

	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(cm.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(cm.client, clusterName)
	require.NoError(cm.T(), err, "Error getting cluster ID")
	cm.cluster, err = cm.client.Management.Cluster.ByID(clusterID)
	require.NoError(cm.T(), err)
}

func (cm *ConfigmapTestSuite) TestConfigMapCreate() {
	subSession := cm.session.NewSession()
	defer subSession.Cleanup()

	configMapClient := cm.client.Steve.SteveType(configmaps.ConfigMapSteveType)
	configmapLabels := map[string]string{labelKey: labelVal}
	configmapData := map[string]string{dataKey: dataVal}

	log.Info("Creating a configmap")
	configmapName := namegenerator.AppendRandomString(cmName)
	configmapObj, err := createConfigmap(*configMapClient, configmapName, namespace, nil, configmapLabels, configmapData)
	require.NoError(cm.T(), err)

	log.Info("Validating configmap was created with correct resource values")
	labels := getConfigMapLabelsAndAnnotations(configmapObj.ObjectMeta.Labels)
	assert.Contains(cm.T(), configmapObj.Name, configmapName)
	assert.Equal(cm.T(), labels, configmapLabels)
}

func (cm *ConfigmapTestSuite) TestConfigMapUpdate() {
	subSession := cm.session.NewSession()
	defer subSession.Cleanup()

	configMapClient := cm.client.Steve.SteveType(configmaps.ConfigMapSteveType)
	configmapLabels := map[string]string{labelKey: labelVal}
	configmapData := map[string]string{dataKey: dataVal}
	actualAnnotations := map[string]string{annoKey: annoVal, descKey: descVal}

	log.Info(("Creating a configmap"))
	configmapName := namegenerator.AppendRandomString(cmName)
	configmapObj, err := createConfigmap(*configMapClient, configmapName, namespace, nil, configmapLabels, configmapData)
	require.NoError(cm.T(), err)

	log.Info("Updating the configmap")
	updatedConfigMapObj, err := updateConfigmapAnnotations(*configMapClient, configmapObj, actualAnnotations)
	require.NoError(cm.T(), err)

	log.Info("Validating configmap was properly updated")
	//annotations := updatedConfigMapObj.Annotations
	expectedAnnotations := getConfigMapLabelsAndAnnotations(updatedConfigMapObj.ObjectMeta.Annotations)
	assert.Equal(cm.T(), expectedAnnotations, actualAnnotations)
}

func (cm *ConfigmapTestSuite) TestConfigmapDelete() {
	subSession := cm.session.NewSession()
	defer subSession.Cleanup()

	configMapClient := cm.client.Steve.SteveType(configmaps.ConfigMapSteveType)
	configmapLabels := map[string]string{labelKey: labelVal}
	configmapData := map[string]string{dataKey: dataVal}

	log.Info("Creating a configmap")
	configmapName := namegenerator.AppendRandomString(cmName)
	configmapObj, err := createConfigmap(*configMapClient, configmapName, namespace, nil, configmapLabels, configmapData)
	require.NoError(cm.T(), err)

	log.Info("Deleting the configmap")
	err = configMapClient.Delete(&configmapObj)
	require.NoError(cm.T(), err)

	log.Info("Validating configmap was deleted")
	configmapByID, err := configMapClient.ByID(configmapObj.ID)
	require.Error(cm.T(), err)
	assert.Nil(cm.T(), configmapByID)
	assert.ErrorContains(cm.T(), err, "not found")
}

func TestConfigMapsSuite(t *testing.T) {
	suite.Run(t, new(ConfigmapTestSuite))
}
