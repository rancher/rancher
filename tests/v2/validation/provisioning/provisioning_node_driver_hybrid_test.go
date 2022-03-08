package provisioning

import (
	"context"
	"fmt"
	"testing"

	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RKE2NodeDriverProvisioningTestSuite) setupMixed() {
	r.SetupSuiteLinuxOnly()
}

func (r *RKE2NodeDriverProvisioningTestSuite) ProvisioningRKE2ClusterHybrid(provider Provider) {
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(r.T(), err)

	providerName := " Node Provider: " + provider.Name

	allNodeRolesMixedOS := []map[string]bool{
		{
			"controlplane": true,
			"etcd":         true,
			"worker":       true,
		},
	}

	uniqueNodeRolesMixedOS := []map[string]bool{
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
		{
			"controlplane": false,
			"etcd":         false,
			"worker":       true,
		},
	}

	tests := []struct {
		name       string
		nodeRoles  []map[string]bool
		hasWindows bool
		client     *rancher.Client
	}{
		{"1 Node all roles Admin User + 1 Windows Worker - MixedOS", allNodeRolesMixedOS, true, r.client},
		{"1 Node all roles Standard User + 1 Windows Worker - MixedOS", allNodeRolesMixedOS, true, r.standardUserClient},
		{"3 unique role nodes as Admin User + 1 Windows Worker - MixedOS", uniqueNodeRolesMixedOS, true, r.client},
		{"3 unique role nodes as Standard User + 1 Windows Worker - MixedOS", uniqueNodeRolesMixedOS, true, r.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		for _, kubeVersion := range r.kubernetesVersions {
			name = tt.name + providerName + " Kubernetes version: " + kubeVersion
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					testSession := session.NewSession(r.T())
					defer testSession.Cleanup()

					testSessionClient, err := tt.client.WithSession(testSession)
					require.NoError(r.T(), err)

					clusterName := AppendRandomString(provider.Name)
					generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
					machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

					machineConfigResp, err := machinepools.CreateMachineConfig(provider.MachineConfig, machinePoolConfig, testSessionClient)
					require.NoError(r.T(), err)

					machinePools := machinepools.RKEMachinePoolSetup(tt.nodeRoles, tt.hasWindows, machineConfigResp)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)

					clusterResp, err := clusters.CreateRKE2Cluster(testSessionClient, cluster)
					require.NoError(r.T(), err)

					result, err := r.client.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterName,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					})
					require.NoError(r.T(), err)

					checkFunc := clusters.IsProvisioningClusterReady

					err = wait.WatchWait(result, checkFunc)
					assert.NoError(r.T(), err)
					assert.Equal(r.T(), clusterName, clusterResp.Name)
				})
			}
		}
	}
}

func (r *RKE2NodeDriverProvisioningTestSuite) ProvisioningRKE2ClusterWithDynamicInputHybrid(provider Provider, nodesAndRoles []map[string]bool, hasWindows bool) {
	providerName := " Node Provider: " + provider.Name

	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(r.T(), err)

	tests := []struct {
		name   string
		client *rancher.Client
	}{
		{"Admin User", r.client},
		{"Standard User", r.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		for _, kubeVersion := range r.kubernetesVersions {
			name = tt.name + providerName + " Kubernetes version: " + kubeVersion
			for _, cni := range r.cnis {
				name += " cni: " + cni
				r.Run(name, func() {
					testSession := session.NewSession(r.T())
					defer testSession.Cleanup()

					testSessionClient, err := tt.client.WithSession(testSession)
					require.NoError(r.T(), err)

					clusterName := AppendRandomString(provider.Name)
					generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
					machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

					machineConfigResp, err := machinepools.CreateMachineConfig(provider.MachineConfig, machinePoolConfig, testSessionClient)
					require.NoError(r.T(), err)

					machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, hasWindows, machineConfigResp)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)

					clusterResp, err := clusters.CreateRKE2Cluster(testSessionClient, cluster)
					require.NoError(r.T(), err)

					result, err := r.client.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
						FieldSelector:  "metadata.name=" + clusterName,
						TimeoutSeconds: &defaults.WatchTimeoutSeconds,
					})
					require.NoError(r.T(), err)

					checkFunc := clusters.IsProvisioningClusterReady

					err = wait.WatchWait(result, checkFunc)
					assert.NoError(r.T(), err)
					assert.Equal(r.T(), clusterName, clusterResp.Name)
				})
			}
		}
	}
}

func (r *RKE2NodeDriverProvisioningTestSuite) TestProvisioningHybrid() {
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)

		r.ProvisioningRKE2ClusterHybrid(provider)
	}
}

func (r *RKE2NodeDriverProvisioningTestSuite) TestProvisioningDynamicInputHybrid() {
	nodesAndRoles := NodesAndRolesInput()
	hasWindows := ClusterHasWindowsInput()
	if len(nodesAndRoles) == 0 {
		r.T().Skip()
	}
	if !hasWindows {
		r.T().Logf("mixedOS Windows + Linux test suite was chosen but hasWindows is set to %v, skipping", hasWindows)
		r.T().Skip()
	}

	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.ProvisioningRKE2ClusterWithDynamicInputHybrid(provider, nodesAndRoles, hasWindows)
	}
}

// TestProvisioningTestSuiteHybrid In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestProvisioningTestSuiteHybrid(t *testing.T) {
	suite.Run(t, new(RKE2NodeDriverProvisioningTestSuite))
}
