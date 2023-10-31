//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package crdgeneration

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	v1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/pkg/session"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CRDGenTestSuite struct {
	suite.Suite
	client      *rancher.Client
	session     *session.Session
	cluster     *management.Cluster
	crds        map[string][]string
	clusterAuth bool
	clusterV1   *v1.Cluster
}

func (crd *CRDGenTestSuite) TearDownSuite() {
	crd.session.Cleanup()
}

func (crd *CRDGenTestSuite) SetupSuite() {
	testSession := session.NewSession()
	crd.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(crd.T(), err)

	crd.client = client
	readJson, err := os.ReadFile(crdJSONFilePath)
	require.NoError(crd.T(), err)
	err = json.Unmarshal(readJson, &crd.crds)
	require.NoError(crd.T(), err)

	clusterType, err := clusters.NewClusterMeta(crd.client, crd.clusterName)
	require.NoError(crd.T(), err)
	crd.clusterAuth = false

	kubeClient, err := crd.client.GetKubeAPIProvisioningClient()
	require.NoError(crd.T(), err)

	crd.clusterV1, err = kubeClient.Clusters(fleetlocal).Get(context.TODO(), localCluster, metav1.GetOptions{})
	require.NoError(crd.T(), err)

}

func (crd *CRDGenTestSuite) sequentialTestCRD() {
	crdsList, err := listCRDS(crd.client, localCluster)
	require.NoError(crd.T(), err)
	crd.Run("Verify the count of crds deployed and the crds on the cluster "+crd.clusterName, func() {
		validateCRDList(crd.T(), crdsList, crd.crds, localCluster)
	})
	crd.Run("Verify description fields of crds are non-empty", func() {
		validateCRDDescription(crd.T(), crd.client, crd.clusterV1, localCluster)
	})
	crd.Run("Verify kubectl validate for role templates", func() {
		validateRoleCreation(crd.T(), crd.client, crd.clusterV1, localCluster)
	})
}

func (crd *CRDGenTestSuite) TestCRDGen() {
	subSession := crd.session.NewSession()
	defer subSession.Cleanup()
	crd.sequentialTestCRD()
}

func TestCRDGenTestSuite(t *testing.T) {
	suite.Run(t, new(CRDGenTestSuite))
}
