//go:build validation

package rke2

import (
	"testing"

	"github.com/rancher/rancher/tests/v2/actions/clusters"
	"github.com/rancher/rancher/tests/v2/actions/provisioning"
	"github.com/rancher/rancher/tests/v2/actions/provisioning/permutations"
	"github.com/rancher/rancher/tests/v2/actions/provisioninginput"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	shepherdclusters "github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/kubernetesversions"
	"github.com/rancher/shepherd/extensions/users"
	password "github.com/rancher/shepherd/extensions/users/passwordgenerator"
	"github.com/rancher/shepherd/pkg/config"
	namegen "github.com/rancher/shepherd/pkg/namegenerator"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type RKE2AgentCustomizationTestSuite struct {
	suite.Suite
	client             *rancher.Client
	session            *session.Session
	standardUserClient *rancher.Client
	provisioningConfig *provisioninginput.Config
}

func (r *RKE2AgentCustomizationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2AgentCustomizationTestSuite) SetupSuite() {
	testSession := session.NewSession()
	r.session = testSession
	r.provisioningConfig = new(provisioninginput.Config)
	config.LoadConfig(provisioninginput.ConfigurationFileKey, r.provisioningConfig)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.provisioningConfig.RKE2KubernetesVersions, err = kubernetesversions.Default(r.client, shepherdclusters.RKE2ClusterType.String(), r.provisioningConfig.RKE2KubernetesVersions)
	require.NoError(r.T(), err)

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
	require.NoError(r.T(), err)

	newUser.Password = user.Password

	standardUserClient, err := client.AsUser(newUser)
	require.NoError(r.T(), err)

	r.standardUserClient = standardUserClient
}

func (r *RKE2AgentCustomizationTestSuite) TestProvisioningRKE2ClusterAgentCustomization() {
	productionPool := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	productionPool[0].MachinePoolConfig.Quantity = 3
	productionPool[1].MachinePoolConfig.Quantity = 2
	productionPool[2].MachinePoolConfig.Quantity = 2

	agentCustomization := management.AgentDeploymentCustomization{
		AppendTolerations: []management.Toleration{
			{
				Key:   "TestKeyToleration",
				Value: "TestValueToleration",
			},
		},
		OverrideResourceRequirements: &management.ResourceRequirements{
			Limits: map[string]string{
				"cpu": "750m",
				"mem": "500Mi",
			},
			Requests: map[string]string{
				"cpu": "250m",
			},
		},
		OverrideAffinity: &management.Affinity{
			NodeAffinity: &management.NodeAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []management.PreferredSchedulingTerm{
					{
						Preference: &management.NodeSelectorTerm{
							MatchExpressions: []management.NodeSelectorRequirement{
								{
									Key:      "testAffinityKey",
									Operator: "In",
									Values:   []string{"true"},
								},
							},
						},
						Weight: 100,
					},
				},
			},
		},
	}

	customAgents := []string{"fleet-agent", "cluster-agent"}
	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
		agent        string
	}{
		{"Custom Fleet Agent - Standard User", productionPool, r.standardUserClient, customAgents[0]},
		{"Custom Cluster Agent - Standard User", productionPool, r.standardUserClient, customAgents[1]},
	}

	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)
		r.provisioningConfig.MachinePools = tt.machinePools

		if tt.agent == "fleet-agent" {
			r.provisioningConfig.FleetAgent = &agentCustomization
			r.provisioningConfig.ClusterAgent = nil
		}

		if tt.agent == "cluster-agent" {
			r.provisioningConfig.ClusterAgent = &agentCustomization
			r.provisioningConfig.FleetAgent = nil
		}

		permutations.RunTestPermutations(&r.Suite, tt.name, client, r.provisioningConfig, permutations.RKE2ProvisionCluster, nil, nil)
	}
}

func (r *RKE2AgentCustomizationTestSuite) TestFailureProvisioningRKE2ClusterAgentCustomization() {
	productionPool := []provisioninginput.MachinePools{provisioninginput.EtcdMachinePool, provisioninginput.ControlPlaneMachinePool, provisioninginput.WorkerMachinePool}
	productionPool[0].MachinePoolConfig.Quantity = 3
	productionPool[1].MachinePoolConfig.Quantity = 2
	productionPool[2].MachinePoolConfig.Quantity = 2

	agentCustomization := management.AgentDeploymentCustomization{
		AppendTolerations: []management.Toleration{
			{
				Key:   "BadLabel",
				Value: "123\"[];'{}-+=",
			},
		},
		OverrideAffinity:             &management.Affinity{},
		OverrideResourceRequirements: &management.ResourceRequirements{},
	}

	customAgents := []string{"fleet-agent", "cluster-agent"}
	tests := []struct {
		name         string
		machinePools []provisioninginput.MachinePools
		client       *rancher.Client
		agent        string
	}{
		{"Invalid Custom Fleet Agent - Standard User", productionPool, r.standardUserClient, customAgents[0]},
		{"Invalid Custom Cluster Agent - Standard User", productionPool, r.standardUserClient, customAgents[1]},
	}

	for _, tt := range tests {
		subSession := r.session.NewSession()
		defer subSession.Cleanup()

		client, err := tt.client.WithSession(subSession)
		require.NoError(r.T(), err)
		r.provisioningConfig.MachinePools = tt.machinePools

		if tt.agent == "fleet-agent" {
			r.provisioningConfig.FleetAgent = &agentCustomization
			r.provisioningConfig.ClusterAgent = nil
		}

		if tt.agent == "cluster-agent" {
			r.provisioningConfig.ClusterAgent = &agentCustomization
			r.provisioningConfig.FleetAgent = nil
		}

		rke2Provider, _, _, kubeVersions := permutations.GetClusterProvider(permutations.RKE2ProvisionCluster, r.provisioningConfig.Providers[0], r.provisioningConfig)
		testClusterConfig := clusters.ConvertConfigToClusterConfig(r.provisioningConfig)
		testClusterConfig.KubernetesVersion = kubeVersions[0]

		_, err = provisioning.CreateProvisioningCluster(client, *rke2Provider, testClusterConfig, nil)
		require.Error(r.T(), err)
	}
}

func TestRKE2AgentCustomizationTestSuite(t *testing.T) {
	suite.Run(t, new(RKE2AgentCustomizationTestSuite))
}
