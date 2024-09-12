//go:build (validation || infra.any || cluster.any || sanity) && !stress && !extended

package serviceaccounts

import (
	"testing"
	"strings"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	"github.com/rancher/shepherd/extensions/clusters"

	"github.com/rancher/rancher/tests/v2/actions/serviceaccounts"
	"github.com/rancher/shepherd/pkg/session"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/stretchr/testify/assert"
)

type SATestSuite struct {
	suite.Suite
	client  *rancher.Client
	session *session.Session
	cluster *management.Cluster
}

func (sa *SATestSuite) TearDownSuite() {
	sa.session.Cleanup()
}

func (sa *SATestSuite) SetupSuite() {
	testSession := session.NewSession()
	sa.session = testSession

	client, err := rancher.NewClient("", testSession)
	require.NoError(sa.T(), err)

	sa.client = client

	log.Info("Getting cluster name from the config file and append cluster details in sa")
	clusterName := client.RancherConfig.ClusterName
	require.NotEmptyf(sa.T(), clusterName, "Cluster name to install should be set")
	clusterID, err := clusters.GetClusterIDByName(sa.client, clusterName)
	require.NoError(sa.T(), err, "Error getting cluster ID")
	sa.cluster, err = sa.client.Management.Cluster.ByID(clusterID)
	require.NoError(sa.T(), err)

}

func getServiceAccounts(rancherClient *rancher.Client, clusterID, namespace string) ([]string, error) {
	steveClient, err := rancherClient.Steve.ProxyDownstream(clusterID)
	if err != nil {
		return nil, err
	}

	//List all serviceaccounts in the given namespace
	serviceAccounts, err := steveClient.SteveType(serviceaccounts.ServiceAccountSteveType).List(nil)
	if err != nil {
		return nil, err
	}

	var filterServiceAccounts []string
	for _, sa := range serviceAccounts.Data {
		if sa.Namespace == namespace {
			filterServiceAccounts = append(filterServiceAccounts, sa.Name)
		}
	}

	return filterServiceAccounts, nil
}

func (sa *SATestSuite) testGetServiceAccountTest(namespace, serviceAccountName string) {
	log.Info("Getting all serviceaccounts in provided namespace")
	serviceAccounts, err := getServiceAccounts(sa.client, sa.cluster.ID, namespace)
	assert.NoError(sa.T(), err)

	switch namespace {
	case "default":
		log.Printf("Verifying serviceaccount '%s' is NOT present in the '%s' namespace", serviceAccountName, namespace)
		assert.NotContains(sa.T(), serviceAccounts, serviceAccountName, "Service account '%s' should not be present in the '%s' namespace", serviceAccountName, namespace)
		log.Info("Check against default namespace is successful")
	case "cattle-system":
		log.Printf("Verifying serviceaccount '%s' is present in the '%s' namespace", serviceAccountName, namespace)
		assert.Contains(sa.T(), serviceAccounts, serviceAccountName, "Service account '%s' should be present in the '%s' namespace", serviceAccountName, namespace)
		log.Info("Check against cattle-system namespace is successful")
	}
}

func (sa *SATestSuite) TestSAs() {
	tests := []struct {
		namespace          string
		serviceAccountName string
	}{
		{"default", "netes-default"},
		{"cattle-system", "kontainer-engine"},
	}
	if !(strings.Contains(sa.cluster.ID, "c-m-")) {
		for _, tt := range tests {
			sa.Run(tt.namespace, func() {
				sa.testGetServiceAccountTest(tt.namespace, tt.serviceAccountName)
			})
		}
	} else {
		log.Info("Serviceaccount test requires an rke1 cluster.")
		sa.T().Skip()
	}
}

func TestSATestSuite(t *testing.T) {
	suite.Run(t, new(SATestSuite))
}
