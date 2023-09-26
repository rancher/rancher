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
	clusterName string
	clusterID   string
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
	crd.clusterName = client.RancherConfig.ClusterName
	require.NotEmptyf(crd.T(), crd.clusterName, "Cluster name to install should be set")
	crd.clusterID, err = clusters.GetClusterIDByName(crd.client, crd.clusterName)
	require.NoError(crd.T(), err, "Error getting cluster ID")
	crd.cluster, err = crd.client.Management.Cluster.ByID(crd.clusterID)
	require.NoError(crd.T(), err)

	readJson, err := os.ReadFile(crdJSONFilePath)
	require.NoError(crd.T(), err)
	err = json.Unmarshal(readJson, &crd.crds)
	require.NoError(crd.T(), err)

	clusterType, err := clusters.NewClusterMeta(crd.client, crd.clusterName)
	require.NoError(crd.T(), err)
	//Change "local" to isLocal for Q4
	if crd.clusterName != "local" && clusterType.Provider == clusters.KubernetesProviderRKE {
		crd.clusterAuth = crd.cluster.LocalClusterAuthEndpoint != nil && crd.cluster.LocalClusterAuthEndpoint.Enabled
	} else {
		crd.clusterAuth = false
	}

	kubeClient, err := crd.client.GetKubeAPIProvisioningClient()
	require.NoError(crd.T(), err)
	fleetNamespace := map[bool]string{crd.clusterName == "local": fleetlocal, true: fleetdefault}[true]

	crd.clusterV1, err = kubeClient.Clusters(fleetNamespace).Get(context.TODO(), crd.clusterName, metav1.GetOptions{})
	require.NoError(crd.T(), err)

}

func (crd *CRDGenTestSuite) sequentialTestCRD() {
	crdsList, err := listCRDS(crd.client, crd.clusterID)
	require.NoError(crd.T(), err)
	crd.Run("Verify the count of crds deployed and the crds on the cluster " +crd.clusterName, func() {
		validateCRDList(crd.T(), crdsList, crd.crds, crd.clusterName)
	})
	crd.Run("Verify description fields of crds are non-empty", func() {
		validateCRDDescription(crd.T(), crd.client, crd.clusterV1, crd.clusterID)
	})
	crd.Run("Verify kubectl validate for role templates", func() {
		validateRoleCreation(crd.T(), crd.client, crd.clusterV1, crd.clusterID)
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
