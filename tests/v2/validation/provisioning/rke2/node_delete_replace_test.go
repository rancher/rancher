package rke2

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	v1 "github.com/rancher/rancher/tests/framework/clients/rancher/v1"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	nodestat "github.com/rancher/rancher/tests/framework/extensions/nodes"
	psadeploy "github.com/rancher/rancher/tests/framework/extensions/psact"
	"github.com/rancher/rancher/tests/framework/extensions/workloads/pods"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/v2/validation/provisioning"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	etcdLabel                = "rke.cattle.io/etcd-role"
	workerLabel              = "rke.cattle.io/worker-role"
	controlPlaneLabel        = "rke.cattle.io/control-plane-role"
	clusterLabel             = "cluster.x-k8s.io/cluster-name"
	machineSteveResourceType = "cluster.x-k8s.io.machine"
)

type NodeDeleteAndReplace struct {
	suite.Suite
	session                *session.Session
	client                 *rancher.Client
	ns                     string
	rke2kubernetesVersions []string
	cnis                   []string
	providers              []string
	nodesAndRoles          []machinepools.NodeRoles
	psact                  string
	advancedOptions        provisioning.AdvancedOptions
}

func (e *NodeDeleteAndReplace) TearDownSuite() {
	e.session.Cleanup()
}

func (e *NodeDeleteAndReplace) SetupSuite() {
	testSession := session.NewSession()
	e.session = testSession

	e.ns = provisioning.Namespace

	clustersConfig := new(provisioning.Config)
	config.LoadConfig(provisioning.ConfigurationFileKey, clustersConfig)

	e.rke2kubernetesVersions = clustersConfig.RKE2KubernetesVersions

	e.cnis = clustersConfig.CNIs
	e.providers = clustersConfig.Providers
	e.nodesAndRoles = clustersConfig.NodesAndRoles
	e.psact = clustersConfig.PSACT
	e.advancedOptions = clustersConfig.AdvancedOptions

	client, err := rancher.NewClient("", testSession)
	require.NoError(e.T(), err)

	e.client = client
}

func (e *NodeDeleteAndReplace) TestNodeDeletionAndReplacement() {
	require.GreaterOrEqual(e.T(), len(e.providers), 1)
	require.GreaterOrEqual(e.T(), len(e.cnis), 1)
	require.GreaterOrEqual(e.T(), len(e.rke2kubernetesVersions), 1)

	subSession := e.session.NewSession()
	defer subSession.Cleanup()

	client, err := e.client.WithSession(subSession)
	require.NoError(e.T(), err)

	for _, provider := range e.providers {
		for _, k8sVersion := range e.rke2kubernetesVersions {
			for _, cni := range e.cnis {
				provider := CreateProvider(provider)
				clusterResp := TestProvisioningRKE2Cluster(e.T(), client, provider, e.nodesAndRoles, k8sVersion, cni, e.psact, e.advancedOptions)

				query, err := url.ParseQuery(fmt.Sprintf("fieldSelector=metadata.namespace=%s", e.ns))
				require.NoError(e.T(), err)

				machines, err := e.client.Steve.SteveType(machineSteveResourceType).List(query)
				require.NoError(e.T(), err)

				previousNodeName := ""
				for _, nodeLabel := range []string{etcdLabel, workerLabel, controlPlaneLabel} {
					numNodesBeforeDeletions := 0
					nodeToDelete := v1.SteveAPIObject{}

					for _, machine := range machines.Data {
						if machine.Labels[nodeLabel] == "true" && machine.Labels[clusterLabel] == clusterResp.Name {
							if machine.Name != previousNodeName {
								nodeToDelete = machine
							}
							numNodesBeforeDeletions++
						}
					}
					previousNodeName = nodeToDelete.Name

					logrus.Info(fmt.Sprintf("Deleting %s node: %s ...", nodeLabel, nodeToDelete.Name))
					err = e.client.Steve.SteveType(machineSteveResourceType).Delete(&nodeToDelete)
					require.NoError(e.T(), err)

					clusterID, err := clusters.GetClusterIDByName(client, clusterResp.Name)
					require.NoError(e.T(), err)

					err = clusters.WaitClusterToBeUpgraded(client, clusterID)
					require.NoError(e.T(), err)

					err = nodestat.IsNodeReady(client, clusterID)
					require.NoError(e.T(), err)

					isNodeReplaced, err := nodestat.IsRKE2K3SNodeReplaced(client, query, clusterResp.Name, nodeLabel, nodeToDelete, numNodesBeforeDeletions)
					require.NoError(e.T(), err)
					require.True(e.T(), isNodeReplaced)

					if e.psact == string(provisioning.RancherPrivileged) || e.psact == string(provisioning.RancherRestricted) {
						err = psadeploy.CheckPSACT(client, clusterResp.Name)
						require.NoError(e.T(), err)

						_, err = psadeploy.CreateNginxDeployment(client, clusterID, e.psact)
						require.NoError(e.T(), err)
					}

					podResults, podErrors := pods.StatusPods(client, clusterID)
					assert.NotEmpty(e.T(), podResults)
					assert.Empty(e.T(), podErrors)

					clusterToken, err := clusters.CheckServiceAccountTokenSecret(client, clusterResp.Name)
					assert.NotEmpty(e.T(), clusterToken)
					require.NoError(e.T(), err)

				}
			}
		}
	}
}

func TestNodeDeleteAndReplace(t *testing.T) {
	suite.Run(t, new(NodeDeleteAndReplace))
}
