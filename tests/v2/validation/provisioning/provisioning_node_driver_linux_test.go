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

func (r *RKE2NodeDriverProvisioningTestSuite) setupLinux() {
	r.SetupSuiteLinuxOnly()
}

func (r *RKE2NodeDriverProvisioningTestSuite) ProvisioningRKE2ClusterLinuxOnly(provider Provider) {
	subSession := r.session.NewSession()
	defer subSession.Cleanup()

	client, err := r.client.WithSession(subSession)
	require.NoError(r.T(), err)

	cloudCredential, err := provider.CloudCredFunc(client)
	require.NoError(r.T(), err)

	allNodeRolesLinux := []map[string]bool{
		{
			"controlplane": true,
			"etcd":         true,
			"worker":       true,
		},
	}

	uniqueNodeRolesLinux := []map[string]bool{
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

	tests := []struct {
		name       string
		nodeRoles  []map[string]bool
		hasWindows bool
		client     *rancher.Client
	}{
		{"1 Node all roles Admin User - Linux", allNodeRolesLinux, false, r.client},
		{"1 Node all roles Standard User - Linux", allNodeRolesLinux, false, r.standardUserClient},
		{"3 nodes - 1 role per node Admin User - Linux", uniqueNodeRolesLinux, false, r.client},
		{"3 nodes - 1 role per node Standard User - Linux", uniqueNodeRolesLinux, false, r.standardUserClient},
	}

	var name string
	for _, tt := range tests {
		for _, kubeVersion := range r.kubernetesVersions {
			name = tt.name + " Kubernetes version: " + kubeVersion
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

					result, err := testSessionClient.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
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

func (r *RKE2NodeDriverProvisioningTestSuite) ProvisioningRKE2ClusterWithDynamicInputLinuxOnly(provider Provider, nodesAndRoles []map[string]bool) {
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
			name = tt.name + " Kubernetes version: " + kubeVersion
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

					machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, false, machineConfigResp)

					cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, cni, cloudCredential.ID, kubeVersion, machinePools)

					clusterResp, err := clusters.CreateRKE2Cluster(testSessionClient, cluster)
					require.NoError(r.T(), err)

					result, err := testSessionClient.Provisioning.Clusters(namespace).Watch(context.TODO(), metav1.ListOptions{
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

func (r *RKE2NodeDriverProvisioningTestSuite) TestProvisioningLinuxOnly() {
	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.ProvisioningRKE2ClusterLinuxOnly(provider)
	}
}

func (r *RKE2NodeDriverProvisioningTestSuite) TestProvisioningDynamicInputLinuxOnly() {
	nodesAndRoles := NodesAndRolesInput()
	if len(nodesAndRoles) == 0 {
		r.T().Skip()
	}

	for _, providerName := range r.providers {
		provider := CreateProvider(providerName)
		r.ProvisioningRKE2ClusterWithDynamicInputLinuxOnly(provider, nodesAndRoles)
	}
}

// TestProvisioningTestSuiteLinuxOnly In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestProvisioningTestSuiteLinuxOnly(t *testing.T) {
	suite.Run(t, new(RKE2NodeDriverProvisioningTestSuite))
}
