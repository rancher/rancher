//go:build (validation || infra.any || cluster.any || extended) && !sanity && !stress

package node

import (
	"net/url"
	"testing"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeAnnotationTestSuite struct {
	suite.Suite
	client      *rancher.Client
	steveclient *v1.Client
	session     *session.Session
	cluster     *management.Cluster
	clusterID   string
	query       url.Values
}

func (ann *NodeAnnotationTestSuite) TearDownSuite() {
	ann.session.Cleanup()
}

func (ann *NodeAnnotationTestSuite) SetupSuite() {
	ann.session = session.NewSession()

	client, err := rancher.NewClient("", ann.session)
	require.NoError(ann.T(), err)

	ann.client = client
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(ann.T(), clusterName, "Cluster name should be set")

	ann.clusterID, err = clusters.GetClusterIDByName(ann.client, clusterName)
	require.NoError(ann.T(), err, "Error getting cluster ID")

	ann.cluster, err = ann.client.Management.Cluster.ByID(ann.clusterID)
	require.NoError(ann.T(), err)

	ann.steveclient, err = ann.client.Steve.ProxyDownstream(ann.clusterID)
	require.NoError(ann.T(), err)

	ann.query, err = url.ParseQuery("labelSelector=node-role.kubernetes.io/worker=true")
	require.NoError(ann.T(), err)
}

func (ann *NodeAnnotationTestSuite) TestAddAnnotation() {

	log.Info("Verify user is able to add annotation on the node through API")

	nodeList, err := ann.steveclient.SteveType("node").List(ann.query)
	require.NoError(ann.T(), err)

	nodeSpec, err := getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	key := namegen.AppendRandomString("key")
	value := namegen.AppendRandomString("value")

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key, value)
	require.NoError(ann.T(), err)

	result, nodeList, err := verifyAnnotation(ann.query, ann.steveclient, true, key, value)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key)
	require.NoError(ann.T(), err)

	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key, value)
	require.NoError(ann.T(), err)

}

func (ann *NodeAnnotationTestSuite) TestAddMultipleAnnotation() {

	log.Info("Verify user is able to add multiple annotation on the node through API")

	nodeList, err := ann.steveclient.SteveType("node").List(ann.query)
	require.NoError(ann.T(), err)

	nodeSpec, err := getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	key1 := namegen.AppendRandomString("key")
	value1 := namegen.AppendRandomString("value")

	key2 := namegen.AppendRandomString("key")
	value2 := namegen.AppendRandomString("value")

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key1, value1)
	require.NoError(ann.T(), err)
	result, nodeList, err := verifyAnnotation(ann.query, ann.steveclient, true, key1, value1)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key2, value2)
	require.NoError(ann.T(), err)
	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, true, key2, value2)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key1)
	require.NoError(ann.T(), err)
	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key1, value1)

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	require.NoError(ann.T(), err)
	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key2)
	require.NoError(ann.T(), err)

	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key2, value2)
	require.NoError(ann.T(), err)

}

func (ann *NodeAnnotationTestSuite) TestEditAnnotation() {

	log.Info("Verify user is able to edit annotation on the node through API")

	nodeList, err := ann.steveclient.SteveType("node").List(ann.query)
	require.NoError(ann.T(), err)

	nodeSpec, err := getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	key := namegen.AppendRandomString("key")
	value := namegen.AppendRandomString("value")

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key, value)
	require.NoError(ann.T(), err)

	result, nodeList, err := verifyAnnotation(ann.query, ann.steveclient, true, key, value)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	newValue := namegen.AppendRandomString("value")

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key, newValue)
	require.NoError(ann.T(), err)

	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, true, key, newValue)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key)
	require.NoError(ann.T(), err)

	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key, newValue)
	require.NoError(ann.T(), err)

}

func (ann *NodeAnnotationTestSuite) TestDeleteAnnotation() {

	log.Info("Verify user is able to delete annotation on the node through API")

	nodeList, err := ann.steveclient.SteveType("node").List(ann.query)
	require.NoError(ann.T(), err)

	nodeSpec, err := getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	key := namegen.AppendRandomString("key")
	value := namegen.AppendRandomString("value")

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key, value)
	require.NoError(ann.T(), err)

	result, nodeList, err := verifyAnnotation(ann.query, ann.steveclient, true, key, value)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key)
	require.NoError(ann.T(), err)

	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key, value)
	require.NoError(ann.T(), err)

}

func (ann *NodeAnnotationTestSuite) TestDeleteMultipleAnnotation() {

	log.Info("Verify user is able to delete multiple annotation on the node through API")

	nodeList, err := ann.steveclient.SteveType("node").List(ann.query)
	require.NoError(ann.T(), err)

	nodeSpec, err := getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	key1 := namegen.AppendRandomString("key")
	value1 := namegen.AppendRandomString("value")

	key2 := namegen.AppendRandomString("key")
	value2 := namegen.AppendRandomString("value")

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key1, value1)
	require.NoError(ann.T(), err)
	result, nodeList, err := verifyAnnotation(ann.query, ann.steveclient, true, key1, value1)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = addAnnotation(nodeSpec, nodeList, ann.steveclient, key2, value2)
	require.NoError(ann.T(), err)
	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, true, key2, value2)
	require.NoError(ann.T(), err)
	assert.Falsef(ann.T(), result == false, "Annotation is not added on the node")

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key1)
	require.NoError(ann.T(), err)
	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key1, value1)

	nodeSpec, err = getNodeSpec(nodeList)
	require.NoError(ann.T(), err)

	require.NoError(ann.T(), err)
	err = deleteAnnotation(nodeSpec, nodeList, ann.steveclient, key2)
	require.NoError(ann.T(), err)

	result, nodeList, err = verifyAnnotation(ann.query, ann.steveclient, false, key2, value2)
	require.NoError(ann.T(), err)

}

func TestNodeAnnotationTestSuite(t *testing.T) {
	suite.Run(t, new(NodeAnnotationTestSuite))
}
