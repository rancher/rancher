package rke1

import (
	"testing"

	"github.com/rancher/norman/types"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	management "github.com/rancher/rancher/tests/framework/clients/rancher/generated/management/v3"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	nodepools "github.com/rancher/rancher/tests/framework/extensions/rke1/nodepools"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EtcdNodeDeleteAndReplace struct {
	suite.Suite
	session                *session.Session
	client                 *rancher.Client
	ns                     string
	rke1kubernetesVersions []string
	cnis                   []string
	providers              []string
	nodesAndRoles          []nodepools.NodeRoles
	psact                  string
	advancedOptions        provisioning.AdvancedOptions
}

func (e *EtcdNodeDeleteAndReplace) TearDownSuite() {
	e.session.Cleanup()
}

func (e *EtcdNodeDeleteAndReplace) SetupSuite() {
	testSession := session.NewSession()
	e.session = testSession

	e.ns = provisioning.Namespace

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	e.rke1kubernetesVersions = clustersConfig.RKE1KubernetesVersions

	e.cnis = clustersConfig.CNIs
	e.providers = clustersConfig.Providers
	e.nodesAndRoles = clustersConfig.NodesAndRolesRKE1
	e.psact = clustersConfig.PSACT
	e.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(e.T(), err)

	e.client = client
}

func (e *EtcdNodeDeleteAndReplace) TestEtcdNodeDeletionAndReplacement() {
	require.GreaterOrEqual(e.T(), len(e.providers), 1)
	require.GreaterOrEqual(e.T(), len(e.cnis), 1)
	require.GreaterOrEqual(e.T(), len(e.rke1kubernetesVersions), 1)

	subSession := e.session.NewSession()
	defer subSession.Cleanup()

	client, err := e.client.WithSession(subSession)
	require.NoError(e.T(), err)

	for _, provider := range e.providers {
		for _, k8sVersion := range e.rke1kubernetesVersions {
			for _, cni := range e.cnis {
				provider := CreateProvider(provider)
				nodeTemplate, err := provider.NodeTemplateFunc(client)
				require.NoError(e.T(), err)

				clusterResp, err := TestProvisioningRKE1Cluster(e.T(), client, provider, e.nodesAndRoles, e.psact, k8sVersion, cni, nodeTemplate, e.advancedOptions)
				require.NoError(e.T(), err)

				machines, err := e.client.Management.Node.List(&types.ListOpts{Filters: map[string]interface{}{
					"clusterId": clusterResp.ID,
				}})
				require.NoError(e.T(), err)

				numOfEtcdNodesBeforeDeletion := 0
				etcdNodeToDelete := management.Node{}

				for _, machine := range machines.Data {
					if machine.Etcd {
						etcdNodeToDelete = machine
						numOfEtcdNodesBeforeDeletion++
					}
				}

				logrus.Info("Deleting, " + etcdNodeToDelete.NodeName + " etcd node..")
				err = e.client.Management.Node.Delete(&etcdNodeToDelete)
				require.NoError(e.T(), err)

				err = clusters.WaitClusterToBeUpgraded(client, clusterResp.ID)
				require.NoError(e.T(), err)

				err = nodestat.IsNodeReady(client, clusterResp.ID)
				require.NoError(e.T(), err)

				isEtcdNodeReplaced, err := nodestat.IsRKE1EtcdNodeReplaced(client, etcdNodeToDelete, clusterResp, numOfEtcdNodesBeforeDeletion)
				require.NoError(e.T(), err)

				require.True(e.T(), isEtcdNodeReplaced)

				podResults, podErrors := pods.StatusPods(client, clusterResp.ID)
				assert.NotEmpty(e.T(), podResults)
				assert.Empty(e.T(), podErrors)

			}
		}
	}
}

func TestEtcdNodeDeleteAndReplace(t *testing.T) {
	suite.Run(t, new(EtcdNodeDeleteAndReplace))
}
