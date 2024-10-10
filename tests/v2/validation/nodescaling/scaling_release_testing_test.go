//go:build validation

package nodescaling

import (
	"slices"
	"strings"
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/machinepools"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/rancher/tests/v2/actions/qaseinput"
	nodepools "github.com/rancher/rancher/tests/v2/actions/rke1/nodepools"
	"github.com/rancher/rancher/tests/v2/actions/scalinginput"
	qase "github.com/rancher/rancher/tests/v2/validation/pipeline/qase/results"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	v1 "github.com/rancher/shepherd/clients/rancher/v1"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NodeScalingReleaseTestingTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	scalingConfig      *scalinginput.Config
	provisioningConfig *provisioninginput.Config
	qaseConfig         *qaseinput.Config
}

func (s *NodeScalingReleaseTestingTestSuite) TearDownSuite() {
	s.session.Cleanup()

	s.qaseConfig = new(qaseinput.Config)
	config.LoadConfig(qaseinput.ConfigurationFileKey, s.qaseConfig)

	if s.qaseConfig.LocalQaseReporting {
		err := qase.ReportTest()
		require.NoError(s.T(), err)
	}
}

func (s *NodeScalingReleaseTestingTestSuite) SetupSuite() {
	testSession := session.NewSession()
	s.session = testSession

	s.scalingConfig = new(scalinginput.Config)
	config.LoadConfig(scalinginput.ConfigurationFileKey, s.scalingConfig)

	s.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, s.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(s.T(), err)

	s.client = client

	enabled := true
	var testuser = namegen.AppendRandomString("testuser-")
	var testpassword = password.GenerateUserPassword("testpass-")
	user := &management.User{
		Username: testuser,
		Password: testpassword,
		Name:     testuser,
		Enabled:  &enabled,
	}

	newUser, err := users.CreateUserWithRole(client, user, "user")
	require.NoError(s.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(s.T(), err)

	s.standardUserClient = standardUserClient
}

func (s *NodeScalingReleaseTestingTestSuite) TestReplacingRKE1Nodes() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	nodeRolesEtcd := nodepools.NodeRoles{
		Etcd: true,
	}

	nodeRolesControlPlane := nodepools.NodeRoles{
		ControlPlane: true,
	}

	nodeRolesWorker := nodepools.NodeRoles{
		Worker: true,
	}

	tests := []struct {
		name      string
		nodeRoles nodepools.NodeRoles
		client    *rancher.Client
	}{
		{"Replacing RKE1 control plane nodes", nodeRolesControlPlane, s.standardUserClient},
		{"Replacing RKE1 etcd nodes", nodeRolesEtcd, s.standardUserClient},
		{"Replacing RKE1 worker nodes", nodeRolesWorker, s.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			_, clusterObject = permutations.RunTestPermutations(&s.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
		}

		s.Run(tt.name, func() {
			adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
			require.NoError(s.T(), err)

			ReplaceRKE1Nodes(s.T(), adminClient, clusterObject.Name, tt.nodeRoles.Etcd, tt.nodeRoles.ControlPlane, tt.nodeRoles.Worker)
		})
	}
}

func (s *NodeScalingReleaseTestingTestSuite) TestReplacingNodes() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	nodeRolesEtcd := machinepools.NodeRoles{
		Etcd: true,
	}

	nodeRolesControlPlane := machinepools.NodeRoles{
		ControlPlane: true,
	}

	nodeRolesWorker := machinepools.NodeRoles{
		Worker: true,
	}

	tests := []struct {
		name      string
		nodeRoles machinepools.NodeRoles
		client    *rancher.Client
	}{
		{"Replacing RKE2 control plane nodes", nodeRolesControlPlane, s.standardUserClient},
		{"Replacing RKE2 etcd nodes", nodeRolesEtcd, s.standardUserClient},
		{"Replacing RKE2 worker nodes", nodeRolesWorker, s.standardUserClient},
		{"Replacing K3S control plane nodes", nodeRolesControlPlane, s.standardUserClient},
		{"Replacing K3S etcd nodes", nodeRolesEtcd, s.standardUserClient},
		{"Replacing K3S worker nodes", nodeRolesWorker, s.standardUserClient},
	}

	var clusterObject *v1.SteveAPIObject
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			if strings.Contains(tt.name, "RKE2") {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
			} else {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
			}
		}

		s.Run(tt.name, func() {
			adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
			require.NoError(s.T(), err)

			ReplaceNodes(s.T(), adminClient, clusterObject.Name, tt.nodeRoles.Etcd, tt.nodeRoles.ControlPlane, tt.nodeRoles.Worker)
		})
	}
}

func (s *NodeScalingReleaseTestingTestSuite) TestScalingRKE1NodePools() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	nodeRolesEtcd := nodepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := nodepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesWorker := nodepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	}

	tests := []struct {
		name      string
		nodeRoles nodepools.NodeRoles
		client    *rancher.Client
	}{
		{"Scaling RKE1 control plane by 1", nodeRolesControlPlane, s.standardUserClient},
		{"Scaling RKE1 etcd node by 1", nodeRolesEtcd, s.standardUserClient},
		{"Scaling RKE1 worker by 1", nodeRolesWorker, s.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			_, clusterObject = permutations.RunTestPermutations(&s.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1ProvisionCluster, nil, nil)
		}

		s.Run(tt.name, func() {
			adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
			require.NoError(s.T(), err)

			clusterID, err := clusters.GetClusterIDByName(adminClient, clusterObject.Name)
			require.NoError(s.T(), err)

			scalingRKE1NodePools(s.T(), adminClient, clusterID, tt.nodeRoles)
		})
	}
}

func (s *NodeScalingReleaseTestingTestSuite) TestScalingMachinePools() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	nodeRolesEtcd := machinepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := machinepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesWorker := machinepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	}

	nodeRolesWindows := machinepools.NodeRoles{
		Windows:  true,
		Quantity: 1,
	}

	tests := []struct {
		name      string
		client    *rancher.Client
		isWindows bool
		nodeRoles machinepools.NodeRoles
	}{
		{"Scaling RKE2 control plane by 1", s.standardUserClient, false, nodeRolesControlPlane},
		{"Scaling RKE2 etcd by 1", s.standardUserClient, false, nodeRolesEtcd},
		{"Scaling RKE2 worker by 1", s.standardUserClient, false, nodeRolesWorker},
		{"Scaling RKE2 Windows worker by 1", s.standardUserClient, true, nodeRolesWindows},
		{"Scaling K3S control plane by 1", s.standardUserClient, false, nodeRolesControlPlane},
		{"Scaling K3S etcd by 1", s.standardUserClient, false, nodeRolesEtcd},
		{"Scaling K3S worker by 1", s.standardUserClient, false, nodeRolesWorker},
	}

	var clusterObject *v1.SteveAPIObject
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			if strings.Contains(tt.name, "RKE2") {
				if !slices.Contains(s.provisioningConfig.Providers, "vsphere") && tt.isWindows {
					s.T().Skip("Windows test requires access to vSphere")
				}

				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
			} else {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SProvisionCluster, nil, nil)
			}
		}

		s.Run(tt.name, func() {
			adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
			require.NoError(s.T(), err)

			clusterID, err := clusters.GetV1ProvisioningClusterByName(adminClient, clusterObject.Name)
			require.NoError(s.T(), err)

			scalingRKE2K3SNodePools(s.T(), adminClient, clusterID, tt.nodeRoles)
		})
	}
}

func (s *NodeScalingReleaseTestingTestSuite) TestScalingRKE1CustomClusterNodes() {
	nodeRolesDedicated := []provisioninginput.NodePools{
		provisioninginput.EtcdMultipleNodes,
		provisioninginput.ControlPlaneMultipleNodes,
		provisioninginput.WorkerMultipleNodes,
	}

	nodeRolesEtcd := nodepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := nodepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesEtcdControlPlane := nodepools.NodeRoles{
		Etcd:         true,
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesWorker := nodepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	}

	tests := []struct {
		name      string
		nodeRoles nodepools.NodeRoles
		client    *rancher.Client
	}{
		{"Scaling RKE1 custom control plane by 1", nodeRolesControlPlane, s.standardUserClient},
		{"Scaling RKE1 custom etcd by 1", nodeRolesEtcd, s.standardUserClient},
		{"Scaling RKE1 custom etcd and control plane by 1", nodeRolesEtcdControlPlane, s.standardUserClient},
		{"Scaling RKE1 custom worker by 1", nodeRolesWorker, s.standardUserClient},
	}

	var clusterObject *management.Cluster
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.NodePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			_, clusterObject = permutations.RunTestPermutations(&s.Suite, "Provision RKE1", client, &provisioningConfig, permutations.RKE1CustomCluster, nil, nil)
		}

		s.Run(tt.name, func() {
			adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
			require.NoError(s.T(), err)

			clusterID, err := clusters.GetClusterIDByName(adminClient, clusterObject.Name)
			require.NoError(s.T(), err)

			scalingRKE1CustomClusterPools(s.T(), adminClient, clusterID, s.scalingConfig.NodeProvider, tt.nodeRoles)
		})
	}
}

func (s *NodeScalingReleaseTestingTestSuite) TestScalingCustomClusterNodes() {
	nodeRolesDedicated := []provisioninginput.MachinePools{
		provisioninginput.EtcdMultipleMachines,
		provisioninginput.ControlPlaneMultipleMachines,
		provisioninginput.WorkerMultipleMachines,
	}

	nodeRolesEtcd := machinepools.NodeRoles{
		Etcd:     true,
		Quantity: 1,
	}

	nodeRolesControlPlane := machinepools.NodeRoles{
		ControlPlane: true,
		Quantity:     1,
	}

	nodeRolesWorker := machinepools.NodeRoles{
		Worker:   true,
		Quantity: 1,
	}

	nodeRolesWindows := machinepools.NodeRoles{
		Windows:  true,
		Quantity: 1,
	}

	tests := []struct {
		name      string
		client    *rancher.Client
		isWindows bool
		nodeRoles machinepools.NodeRoles
	}{
		{"Scaling custom RKE2 control plane by 1", s.standardUserClient, false, nodeRolesControlPlane},
		{"Scaling custom RKE2 etcd by 1", s.standardUserClient, false, nodeRolesEtcd},
		{"Scaling custom RKE2 worker by 1", s.standardUserClient, false, nodeRolesWorker},
		{"Scaling custom RKE2 Windows worker by 1", s.standardUserClient, true, nodeRolesWindows},
		{"Scaling custom K3S control plane by 1", s.standardUserClient, false, nodeRolesControlPlane},
		{"Scaling custom K3S etcd by 1", s.standardUserClient, false, nodeRolesEtcd},
		{"Scaling custom K3S worker by 1", s.standardUserClient, false, nodeRolesWorker},
	}

	var clusterObject *v1.SteveAPIObject
	provisioningConfig := *s.provisioningConfig
	provisioningConfig.MachinePools = nodeRolesDedicated

	for _, tt := range tests {
		subSession := s.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(s.T(), err)

		if clusterObject == nil {
			if strings.Contains(tt.name, "RKE2") {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision RKE2", client, &provisioningConfig, permutations.RKE2CustomCluster, nil, nil)
			} else {
				clusterObject, _ = permutations.RunTestPermutations(&s.Suite, "Provision K3S", client, &provisioningConfig, permutations.K3SCustomCluster, nil, nil)
			}
		}

		s.Run(tt.name, func() {
			adminClient, err := rancher.NewClient(tt.client.RancherConfig.AdminToken, client.Session)
			require.NoError(s.T(), err)

			clusterID, err := clusters.GetV1ProvisioningClusterByName(adminClient, clusterObject.Name)
			require.NoError(s.T(), err)

			scalingRKE2K3SCustomClusterPools(s.T(), adminClient, clusterID, s.scalingConfig.NodeProvider, tt.nodeRoles)
		})
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestNodeScalingReleaseTestingTestSuite(t *testing.T) {
	suite.Run(t, new(NodeScalingReleaseTestingTestSuite))
}
