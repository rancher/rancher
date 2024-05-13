package deleting

import (
	"strings"
	"testing"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	ProvisioningSteveResourceType = "provisioning.cattle.io.cluster"
)

type DeleteInitMachineTestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
}

func (d *DeleteInitMachineTestSuite) TearDownSuite() {
	d.session.Cleanup()
}

func (d *DeleteInitMachineTestSuite) SetupSuite() {
	testSession := session.NewSession()
	d.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(d.T(), err)

	d.client = client
}

func (d *DeleteInitMachineTestSuite) TestDeleteInitMachine() {
	clusterID, err := clusters.GetV1ProvisioningClusterByName(d.client, d.client.RancherConfig.ClusterName)
	require.NoError(d.T(), err)

	cluster, err := d.client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
	require.NoError(d.T(), err)

	updatedCluster := new(apisV1.Cluster)
	err = v1.ConvertToK8sType(cluster, &updatedCluster)
	require.NoError(d.T(), err)

	name := "K3S"

	if strings.Contains(updatedCluster.Spec.KubernetesVersion, "rke2") {
		name = "RKE2"
	}
	require.NotContains(d.T(), updatedCluster.Spec.KubernetesVersion, "-rancher")
	require.NotEmpty(d.T(), updatedCluster.Spec.KubernetesVersion)

	d.Run(name, func() {
		deleteInitMachine(d.T(), d.client, clusterID)
	})
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestDeleteInitMachineTestSuite(t *testing.T) {
	suite.Run(t, new(DeleteInitMachineTestSuite))
}