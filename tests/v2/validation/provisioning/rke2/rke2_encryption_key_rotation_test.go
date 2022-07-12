package rke2

import (
	"context"
	"fmt"
	"strings"
	"testing"

	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	"github.com/rancher/rancher/tests/framework/clients/rancher"
	"github.com/rancher/rancher/tests/framework/extensions/cloudcredentials"
	"github.com/rancher/rancher/tests/framework/extensions/clusters"
	"github.com/rancher/rancher/tests/framework/extensions/machinepools"
	"github.com/rancher/rancher/tests/framework/pkg/config"
	"github.com/rancher/rancher/tests/framework/pkg/session"
	"github.com/rancher/rancher/tests/framework/pkg/wait"
	"github.com/rancher/rancher/tests/integration/pkg/defaults"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type RKE2EncryptionKeyRotationTestSuite struct {
	suite.Suite
	session     *session.Session
	client      *rancher.Client
	config      *Config
	clusterName string
	namespace   string
}

var phases = []rkev1.RotateEncryptionKeysPhase{
	rkev1.RotateEncryptionKeysPhasePrepare,
	rkev1.RotateEncryptionKeysPhasePostPrepareRestart,
	rkev1.RotateEncryptionKeysPhaseRotate,
	rkev1.RotateEncryptionKeysPhasePostRotateRestart,
	rkev1.RotateEncryptionKeysPhaseReencrypt,
	rkev1.RotateEncryptionKeysPhasePostReencryptRestart,
	rkev1.RotateEncryptionKeysPhaseDone,
}

func (r *RKE2EncryptionKeyRotationTestSuite) TearDownSuite() {
	r.session.Cleanup()
}

func (r *RKE2EncryptionKeyRotationTestSuite) SetupSuite() {
	testSession := session.NewSession(r.T())
	r.session = testSession

	r.config = new(Config)
	config.LoadConfig(ConfigurationFileKey, r.config)

	client, err := rancher.NewClient("", testSession)
	require.NoError(r.T(), err)

	r.client = client

	r.clusterName = r.client.RancherConfig.ClusterName
	r.namespace = r.client.RancherConfig.ClusterName
}

func (r *RKE2EncryptionKeyRotationTestSuite) TestEncryptionKeyRotationImpl(provider Provider, kubeVersion string, nodesAndRoles []machinepools.NodeRoles, credential *cloudcredentials.CloudCredential) {
	name := fmt.Sprintf("Provider_%s/Kubernetes_Version_%s/Nodes_%v", provider.Name, kubeVersion, nodesAndRoles)
	r.Run(name, func() {
		testSession := session.NewSession(r.T())
		defer testSession.Cleanup()

		testSessionClient, err := r.client.WithSession(testSession)
		require.NoError(r.T(), err)

		clusterName := AppendRandomString(fmt.Sprintf("%s-%s", r.clusterName, provider.Name))
		generatedPoolName := fmt.Sprintf("nc-%s-pool1-", clusterName)
		machinePoolConfig := provider.MachinePoolFunc(generatedPoolName, namespace)

		machineConfigResp, err := machinepools.CreateMachineConfig(provider.MachineConfig, machinePoolConfig, testSessionClient)
		require.NoError(r.T(), err)

		machinePools := machinepools.RKEMachinePoolSetup(nodesAndRoles, machineConfigResp)

		cluster := clusters.NewRKE2ClusterConfig(clusterName, namespace, "calico", credential.ID, kubeVersion, machinePools)

		if strings.Contains(kubeVersion, "k3s") {
			cluster.Spec.RKEConfig.MachineGlobalConfig.Data["secrets-encryption"] = true
		}

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

		provCluster, err := r.client.Provisioning.Clusters(namespace).Get(context.TODO(), clusterName, metav1.GetOptions{})
		assert.NoError(r.T(), err)
		if err != nil {
			return
		}

		provCluster = provCluster.DeepCopy()
		provCluster.Spec.RKEConfig.RotateEncryptionKeys = &rkev1.RotateEncryptionKeys{
			Generation: 1,
		}

		provCluster, err = r.client.Provisioning.Clusters(namespace).Update(context.TODO(), provCluster, metav1.UpdateOptions{})
		assert.NoError(r.T(), err)

		for _, phase := range phases {
			result, err := r.client.RKE.RKEControlPlanes(namespace).Watch(context.TODO(), metav1.ListOptions{
				FieldSelector:  "metadata.name=" + clusterName,
				TimeoutSeconds: &defaults.WatchTimeoutSeconds,
			})
			require.NoError(r.T(), err)

			checkFunc := IsAtLeast(r.T(), phase)

			err = wait.WatchWait(result, checkFunc)
			if !assert.NoError(r.T(), err) {
				break
			}
		}
	})
}

func (r *RKE2EncryptionKeyRotationTestSuite) TestEncryptionKeyRotation() {
	for _, providerName := range r.config.Providers {
		subSession := r.session.NewSession()

		provider := CreateProvider(providerName)

		client, err := r.client.WithSession(subSession)
		require.NoError(r.T(), err)

		cloudCredential, err := provider.CloudCredFunc(client)
		require.NoError(r.T(), err)

		for _, kubernetesVersion := range r.config.KubernetesVersions {
			r.TestEncryptionKeyRotationImpl(provider, kubernetesVersion, r.config.NodesAndRoles, cloudCredential)
		}

		subSession.Cleanup()
	}
}

func IsAtLeast(t *testing.T, phase rkev1.RotateEncryptionKeysPhase) wait.WatchCheckFunc {
	return func(event watch.Event) (ready bool, err error) {
		controlPlane := event.Object.(*rkev1.RKEControlPlane)

		if controlPlane.Status.RotateEncryptionKeysPhase == rkev1.RotateEncryptionKeysPhaseFailed {
			t.Errorf("Encryption key rotation failed waiting to reach %s", phase)
			return ready, fmt.Errorf("encryption key rotation failed")
		}

		desiredPhase := -1
		currentPhase := -1

		for i, v := range phases {
			if v == phase {
				desiredPhase = i
			}
			if v == controlPlane.Status.RotateEncryptionKeysPhase {
				currentPhase = i
			}
			if desiredPhase != -1 && currentPhase != -1 {
				break
			}
		}

		if currentPhase < desiredPhase {
			return false, nil
		}

		t.Logf("Encryption key rotation successfully entered %s", phase)

		return true, nil
	}
}

func TestEncryptionKeyRotation(t *testing.T) {
	suite.Run(t, new(RKE2EncryptionKeyRotationTestSuite))
}
