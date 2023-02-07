package registries

import (
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/registries"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RegistryTestSuite struct {
	suite.Suite
	session                        *session.Session
	podListClusterAuth             *v1.SteveCollection
	podListClusterNoAuth           *v1.SteveCollection
	podListClusterLocal            *v1.SteveCollection
	clusterAuthRegistryHost        string
	clusterNoAuthRegistryHost      string
	localClusterGlobalRegistryHost string
}

func (rt *RegistryTestSuite) TearDownSuite() {
	rt.session.Cleanup()
}

func (rt *RegistryTestSuite) SetupSuite() {
	testSession := session.NewSession()
	rt.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(rt.T(), err)

	registriesConfig := new(Config)
	config.LoadConfig(RegistriesConfigKey, registriesConfig)

	for _, v := range registriesConfig.Clusters {
		clusterID, err := clusters.GetClusterIDByName(client, v.Name)
		require.NoError(rt.T(), err)
		downstreamClient, err := client.Steve.ProxyDownstream(clusterID)
		require.NoError(rt.T(), err)
		steveClient := downstreamClient.SteveType(pods.PodResourceSteveType)

		podsList, err := steveClient.List(nil)
		require.NoError(rt.T(), err)
		if v.Auth == true {
			rt.podListClusterAuth = podsList
			rt.clusterAuthRegistryHost = v.URL
		} else if v.Name == "local" {
			rt.podListClusterLocal = podsList
			rt.localClusterGlobalRegistryHost = v.URL
		} else {
			rt.podListClusterNoAuth = podsList
			rt.clusterNoAuthRegistryHost = v.URL
}		}
	}
}

func (rt *RegistryTestSuite) TestRegistryAllPods() {

	havePrefix, err := registries.CheckAllClusterPodsForRegistryPrefix(rt.podListClusterAuth, rt.clusterAuthRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), havePrefix)

	havePrefix, err = registries.CheckAllClusterPodsForRegistryPrefix(rt.podListClusterNoAuth, rt.clusterNoAuthRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), havePrefix)

	havePrefix, err = registries.CheckAllClusterPodsForRegistryPrefix(rt.podListClusterLocal, rt.localClusterGlobalRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), havePrefix)

}

func (rt *RegistryTestSuite) TestStatusAllPods() {

	areStatusesOk, err := registries.CheckAllClusterPodsStatusForRegistry(rt.podListClusterAuth, rt.clusterAuthRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), areStatusesOk)

	areStatusesOk, err = registries.CheckAllClusterPodsStatusForRegistry(rt.podListClusterNoAuth, rt.clusterNoAuthRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), areStatusesOk)

	areStatusesOk, err = registries.CheckAllClusterPodsStatusForRegistry(rt.podListClusterLocal, rt.localClusterGlobalRegistryHost)
	require.NoError(rt.T(), err)
	assert.True(rt.T(), areStatusesOk)
}

func TestRegistryTestSuite(t *testing.T) {
	suite.Run(t, new(RegistryTestSuite))
}
