package v1_api_tests

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	apisV1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	v3 "github.com/rancher/rancher/pkg/client/generated/management/v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/rancher/rancher/tests/go-validation/aws"

	"github.com/rancher/rancher/tests/go-validation/clients"
	"github.com/rancher/rancher/tests/go-validation/cloudcredentials"
	"github.com/rancher/rancher/tests/go-validation/cluster"
	"github.com/rancher/rancher/tests/go-validation/environmentvariables"
	"github.com/rancher/rancher/tests/go-validation/machinepool"
	"github.com/rancher/rancher/tests/go-validation/namegenerator"
	"github.com/rancher/rancher/tests/go-validation/tokenregistration"
	"github.com/rancher/rancher/tests/go-validation/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	digitalOceanCloudCredentialName = "docloudcredential"
	namespace                       = "fleet-default"
	defaultRandStringLength         = 5
	baseDOClusterName               = "docluster"
	baseCustomClusterName           = "customcluster"
	baseEC2Name                     = "rancherautomation"
	defaultTokenName                = "default-token"
)

var nodesAndRoles = os.Getenv("NODE_ROLES")

func CreateEC2Instances(s *ProvisioningTestSuite) {
	s.T().Log("Creating EC2 Instances")
	s.baseEC2Name = "automation-"
	s.randString = namegenerator.RandStringLowerBytes(5)
	ec2NodeName := s.baseEC2Name + s.randString

	newClient, err := aws.NewEC2Client()
	s.ec2Client = newClient
	require.NoError(s.T(), err)

	s.nodes, err = s.ec2Client.CreateNodes(ec2NodeName, true, s.nodeConfig.NumNodes)
	require.NoError(s.T(), err)
	s.T().Log("Successfully created EC2 Instances")
}

func GenerateClusterName(s *ProvisioningTestSuite) string {
	s.T().Log("Create Cluster")
	//randomize name
	clusterName := s.baseClusterName + namegenerator.RandStringLowerBytes(5)
	return clusterName
}

func MachinePoolSetup(s *ProvisioningTestSuite, nodeRoles []map[string]bool) {
	machinePools := []apisV1.RKEMachinePool{}
	for index, roles := range nodeRoles {
		machinePool := machinepool.MachinePoolSetup(roles["controlplane"], roles["etcd"], roles["worker"], "pool"+strconv.Itoa(index), 1, s.machineConfig)
		machinePools = append(machinePools, machinePool)
	}

	s.rkeMachinePools = machinePools
}

func CreateMachineConfigPool(s *ProvisioningTestSuite) {
	generatedPoolName := fmt.Sprintf("nc-%s-pool1", "digitalocean")

	machinePoolConfig := machinepool.NewMachinePoolConfig(generatedPoolName, machinepool.DOKind, namespace, machinepool.DOPoolType, "ubuntu-20-04-x64", "nyc3", "s-2vcpu-4gb")

	s.T().Logf("Creating DO machine pool config %s", generatedPoolName)
	podConfigClient, err := clients.NewPodConfigClient(clients.DOResourceConfig, s.bearerToken)
	require.NoError(s.T(), err)

	machineConfigResult, err := machinepool.CreateMachineConfigPool(machinePoolConfig, podConfigClient)
	require.NoError(s.T(), err)

	s.T().Logf("Successfully created DO machine pool %s", generatedPoolName)

	s.machineConfig = machineConfigResult
}

func RegisterNodesForCustomCluster(s *ProvisioningTestSuite, clusterGeneratedName string) {
	managementClient, err := clients.NewManagementClient(s.bearerToken)
	s.managementClient = managementClient
	require.NoError(s.T(), err)

	s.T().Log("Client creation was successful")

	clusterRegistrationToken := tokenregistration.NewClusterRegistrationToken(s.managementClient)

	// all registration creation and existence.
	// time.Sleep(10 * time.Second)

	s.T().Log("Before getting registration token")
	token, err := clusterRegistrationToken.GetRegistrationToken(clusterGeneratedName)
	require.NoError(s.T(), err)
	s.T().Log("Successfully got registration token")

	for key, node := range s.nodes {
		s.T().Logf("Execute Registration Command for node %s", node.NodeID)
		command := fmt.Sprintf("%s %s", token.InsecureNodeCommand, s.nodeConfig.Roles[key])

		err = node.ExecuteCommand(command)
		require.NoError(s.T(), err)
	}
}

func ProvisionCluster(s *ProvisioningTestSuite, cloudCredentialName string) {
	s.T().Logf("User is %s", s.bearerName)

	clusterName := GenerateClusterName(s)
	s.clusterNames = append(s.clusterNames, clusterName)

	provisioningClient, err := clients.NewProvisioningClient(s.bearerToken)
	require.NoError(s.T(), err)

	clusterConfig := cluster.NewRKE2ClusterConfig(clusterName, s.clusterNamespace, cluster.CNI, cloudCredentialName, cluster.KubernetesVersion, s.rkeMachinePools)

	clusterObj := cluster.NewCluster(s.clusterNamespace, provisioningClient)
	s.clusterObjs = append(s.clusterObjs, clusterObj)

	s.T().Logf("Creating Cluster %s", clusterName)
	v1Cluster, err := clusterObj.CreateCluster(clusterConfig)
	require.NoError(s.T(), err)

	assert.Equal(s.T(), v1Cluster.Name, clusterName)

	s.T().Logf("Created Cluster %s", v1Cluster.Name)

	getCluster, err := clusterObj.PollCluster(v1Cluster.Name)
	require.NoError(s.T(), err)

	s.v1Cluster = getCluster
}

type ProvisioningTestSuite struct {
	suite.Suite

	// server config
	host       string
	password   string
	adminToken string
	userToken  string
	cni        string
	k8sVersion string

	// Digital Ocean variables
	digitalOceanCloudCredential       *cloudcredentials.CloudCredential
	digitalOceanCloudCredentialConfig *v3.CloudCredential

	// Node Driver variables
	rkeMachinePools []apisV1.RKEMachinePool
	machineConfig   *unstructured.Unstructured

	// AWS environment variables
	awsInstanceType    string
	awsRegion          string
	awsRegionAZ        string
	awsAMI             string
	awsSecurityGroup   string
	awsSSHKeyName      string
	awsCICDInstanceTag string
	awsIAMProfile      string
	awsUser            string
	awsVolumeSize      int

	ec2Client        *aws.EC2Client
	nodes            []*aws.EC2Node
	nodeConfig       *cluster.NodeConfig
	baseEC2Name      string
	baseClusterName  string
	clusterNamespace string
	randString       string

	// rest api variables
	bearerName       string
	bearerToken      string
	managementClient *v3.Client

	clusterNames []string
	clusterObjs  []*cluster.Cluster
	v1Cluster    *apisV1.Cluster

	rancherCleanup bool
}

// The SetupSuite method will be run by testify once, at the very
// start of the testing suite, before any tests are run.
func (s *ProvisioningTestSuite) SetupSuite() {
	s.host = clients.Host
	s.password = user.Password
	s.adminToken = clients.AdminToken
	s.userToken = clients.UserToken
	s.cni = cluster.CNI
	s.k8sVersion = cluster.KubernetesVersion
	s.clusterNamespace = "fleet-default"
	s.rancherCleanup = environmentvariables.RancherCleanup()

	testifyArg := os.Args[5:len(os.Args)]

	var testCase string
	if testifyArg[0] == "-testify.m" {
		testCase = testifyArg[1]
	} else {
		testCase = ""
	}

	if strings.Contains(testCase, "DigitalOcean") || testCase == "" {
		for userName, bearerToken := range clients.BearerTokensList() {

			s.T().Logf("Create Cloud Credential for user %s", userName)
			managementClient, err := clients.NewManagementClient(bearerToken)
			s.managementClient = managementClient
			require.NoError(s.T(), err)

			cloudCredentialName := digitalOceanCloudCredentialName + namegenerator.RandStringLowerBytes(defaultRandStringLength)
			// doCloudCred := cloudcredentials.NewCloudCredentialSecret(cloudCredentialName, "", "digitalocean", namespace)
			doCloudCred := cloudcredentials.NewCloudCredentialConfig(cloudCredentialName, "", "digitalocean", namespace)

			cloudCredential := cloudcredentials.NewCloudCredential(managementClient)
			returnedDoCloudCred, err := cloudCredential.CreateCloudCredential(doCloudCred)
			require.NoError(s.T(), err)

			s.digitalOceanCloudCredential = cloudCredential
			s.digitalOceanCloudCredentialConfig = returnedDoCloudCred
		}
	}
	if strings.Contains(testCase, "CustomCluster") || testCase == "" {
		s.awsInstanceType = aws.AWSInstanceType
		s.awsRegion = aws.AWSRegion
		s.awsRegionAZ = aws.AWSRegionAZ
		s.awsAMI = aws.AWSAMI
		s.awsSecurityGroup = aws.AWSSecurityGroup
		s.awsSSHKeyName = aws.AWSSSHKeyName
		s.awsCICDInstanceTag = aws.AWSCICDInstanceTag
		s.awsIAMProfile = aws.AWSIAMProfile
		s.awsUser = aws.AWSUser
		s.awsVolumeSize = aws.AWSVolumeSize
	}
}

func (s *ProvisioningTestSuite) TestProvisioning_RKE2CustomCluster() {
	roles0 := []string{
		"--etcd --controlplane --worker",
	}

	roles1 := []string{
		"--etcd",
		"--controlplane",
		"--worker",
	}

	tests := []cluster.NodeConfig{
		{Name: "1 Node all roles", NumNodes: 1, Roles: roles0},
		{Name: "3 nodes - 1 role per node", NumNodes: 3, Roles: roles1},
	}
	s.baseClusterName = baseCustomClusterName

	for _, tt := range tests {
		for s.bearerName, s.bearerToken = range clients.BearerTokensList() {
			s.nodeConfig = cluster.NewNodeConfig(tt.Name+" "+s.bearerName, tt.NumNodes, tt.Roles)
			s.Run(s.nodeConfig.Name, func() {
				CreateEC2Instances(s)
				ProvisionCluster(s, "")
				RegisterNodesForCustomCluster(s, s.v1Cluster.Status.ClusterName)

				s.T().Logf("Checking status of cluster %s", s.v1Cluster.GetClusterName())

				//check cluster status
				clusterObj := s.clusterObjs[0]
				ready, err := clusterObj.CheckClusterStatus(s.v1Cluster.Name)
				assert.NoError(s.T(), err)
				assert.True(s.T(), ready)
			})
		}
	}
}

func (s *ProvisioningTestSuite) TestProvisioning_RKE2CustomClusterDynamicInput() {
	if nodesAndRoles == "" {
		s.T().Skip()
	}

	s.baseClusterName = baseCustomClusterName
	rolesPerNode := []string{}
	rolesSlice := strings.Split(nodesAndRoles, "|")
	numNodes := len(rolesSlice)

	for i := 0; i < numNodes; i++ {
		roles := rolesSlice[i]
		roleCommands := strings.Split(roles, ",")
		var finalRoleCommand string
		for _, roleCommand := range roleCommands {
			finalRoleCommand += fmt.Sprintf(" --%s", roleCommand)
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
	}

	s.nodeConfig = cluster.NewNodeConfig("dynamicRoles", int64(numNodes), rolesPerNode)
	for s.bearerName, s.bearerToken = range clients.BearerTokensList() {
		s.Run(s.bearerName, func() {
			CreateEC2Instances(s)
			ProvisionCluster(s, "")
			RegisterNodesForCustomCluster(s, s.v1Cluster.Status.ClusterName)

			s.T().Logf("Checking status of cluster %s", s.v1Cluster.GetClusterName())

			//check cluster status
			clusterObj := s.clusterObjs[0]
			ready, err := clusterObj.CheckClusterStatus(s.v1Cluster.Name)
			assert.NoError(s.T(), err)
			assert.True(s.T(), ready)
		})
	}
}

func (s *ProvisioningTestSuite) TestProvisioning_RKE2DigitalOceanCluster() {
	for s.bearerName, s.bearerToken = range clients.BearerTokensList() {
		nodeRoles0 := []map[string]bool{
			{
				"controlplane": true,
				"etcd":         true,
				"worker":       true,
			},
		}

		nodeRoles1 := []map[string]bool{
			{
				"controlplane": true,
				"etcd":         false,
				"worker":       false,
			},
			{
				"controlplane": false,
				"etcd":         true,
				"worker":       false,
			},
			{
				"controlplane": false,
				"etcd":         false,
				"worker":       true,
			},
		}

		s.baseClusterName = baseDOClusterName

		tests := []struct {
			name      string
			nodeRoles []map[string]bool
		}{
			{"1 Node all roles", nodeRoles0},
			{"3 nodes - 1 role per node", nodeRoles1},
		}

		for _, tt := range tests {
			name := tt.name + " " + s.bearerName
			s.Run(name, func() {
				CreateMachineConfigPool(s)
				MachinePoolSetup(s, tt.nodeRoles)

				ProvisionCluster(s, s.digitalOceanCloudCredentialConfig.ID)

				s.T().Logf("Checking status of cluster %s", s.v1Cluster.Name)
				//check cluster status
				clusterObj := s.clusterObjs[0]
				ready, err := clusterObj.CheckClusterStatus(s.v1Cluster.Name)
				assert.NoError(s.T(), err)
				assert.True(s.T(), ready)
			})
		}
	}
}

func (s *ProvisioningTestSuite) TestProvisioning_RKE2DigitalOceanClusterDynamicInput() {
	if nodesAndRoles == "" {
		s.T().Skip()
	}

	nodeRolesBoolSliceMap := []map[string]bool{}

	rolesSlice := strings.Split(nodesAndRoles, "|")
	for _, roles := range rolesSlice {
		nodeRoles := strings.Split(roles, ",")
		nodeRoleBoolMap := map[string]bool{}
		for _, nodeRole := range nodeRoles {
			nodeRoleBoolMap[nodeRole] = true

		}
		nodeRolesBoolSliceMap = append(nodeRolesBoolSliceMap, nodeRoleBoolMap)
	}

	s.baseClusterName = baseDOClusterName
	for s.bearerName, s.bearerToken = range clients.BearerTokensList() {
		s.Run(s.bearerName, func() {
			CreateMachineConfigPool(s)
			MachinePoolSetup(s, nodeRolesBoolSliceMap)

			ProvisionCluster(s, s.digitalOceanCloudCredentialConfig.ID)

			s.T().Logf("Checking status of cluster %s", s.v1Cluster.Name)
			//check cluster status
			clusterObj := s.clusterObjs[0]
			ready, err := clusterObj.CheckClusterStatus(s.v1Cluster.Name)
			assert.NoError(s.T(), err)
			assert.True(s.T(), ready)
		})
	}
}

// TearDownTest is a teardown after every test
func (s *ProvisioningTestSuite) TearDownTest() {
	s.rkeMachinePools = nil
}

// TearDownSuite tears down the whole suite
func (s *ProvisioningTestSuite) TearDownSuite() {
	if s.rancherCleanup {
		err := cluster.ClusterCleanup(s.ec2Client, s.clusterObjs, s.clusterNames, s.nodes)
		require.NoError(s.T(), err)
		if s.digitalOceanCloudCredential != nil {
			// temporary fix to wait for rancher to delete Digital Ocean droplets before deleting the cloud credential.
			// this is to avoid the situation where deleting the cloud credential too soon would result in the Digital Ocean droplets not being deleted.
			time.Sleep(30 * time.Second)
			err := s.digitalOceanCloudCredential.DeleteCloudCredential(s.digitalOceanCloudCredentialConfig)
			require.NoError(s.T(), err)
		}
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestProvisioningTestSuite(t *testing.T) {
	suite.Run(t, new(ProvisioningTestSuite))
}
