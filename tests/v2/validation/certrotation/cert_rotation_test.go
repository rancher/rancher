//go:build (validation || infra.rke2k3s || cluster.any || stress) && !infra.any && !infra.aks && !infra.eks && !infra.gke && !infra.rke1 && !sanity && !extended

package certrotation

import (
	"strings"
	"testing"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	steveV1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type V2ProvCertRotationTestSuite struct {
	suite.Suite
	session        *session.Session
	client         *rancher.Client
	clustersConfig *provisioninginput.Config
}

func (r *V2ProvCertRotationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *V2ProvCertRotationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession

	r.clustersConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.clustersConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client
}

func (r *V2ProvCertRotationTestSuite) TestCertRotation() {
	id, err := clusters.GetV1ProvisioningClusterByName(r.client, r.client.RancherConfig.ClusterName)
	require.NoError(r.T(), err)

	cluster, err := r.client.Steve.SteveType(provisioningSteveResourceType).ByID(id)
	require.NoError(r.T(), err)

	spec := &provv1.ClusterSpec{}
	err = steveV1.ConvertToK8sType(cluster.Spec, spec)
	require.NoError(r.T(), err)

	clusterType := "RKE1"

	if strings.Contains(spec.KubernetesVersion, "-rancher") || len(spec.KubernetesVersion) == 0 {
		r.Run("test-cert-rotation "+clusterType, func() {
			require.NoError(r.T(), rotateRKE1Certs(r.client, r.client.RancherConfig.ClusterName))
			require.NoError(r.T(), rotateRKE1Certs(r.client, r.client.RancherConfig.ClusterName))
		})
	} else {
		if strings.Contains(spec.KubernetesVersion, "k3s") {
			clusterType = "K3s"
		} else {
			clusterType = "RKE2"
		}

		r.Run("test-cert-rotation "+clusterType, func() {
			require.NoError(r.T(), rotateCerts(r.client, r.client.RancherConfig.ClusterName))
			require.NoError(r.T(), rotateCerts(r.client, r.client.RancherConfig.ClusterName))
		})
	}
}

func TestCertRotationTestSuite(t *testing.T) {
	suite.Run(t, new(V2ProvCertRotationTestSuite))
}
