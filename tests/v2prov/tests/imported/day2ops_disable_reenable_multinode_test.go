package imported

import (
	"context"
	"fmt"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// Test_Imported_Operation_SetD_ImportedDay2OpsDisableReenableSnapshotSave_MultiNode validates imported
// day2ops disable/re-enable lifecycle and ETCDSnapshotSave on a two-node imported RKE2 topology.
func Test_Imported_Operation_SetD_ImportedDay2OpsDisableReenableSnapshotSave_MultiNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	assertImportedDay2OpsFeatureEnabled(t, clients)

	fixture := setUpImportedCluster(t, clients, &v3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "c-",
			Annotations: map[string]string{
				opsEnabledAnnotation: "true",
			},
		},
		Spec: v3.ClusterSpec{
			ImportedConfig: &v3.ImportedConfig{},
			DisplayName:    "test-imported-day2ops-disable-reenable-snapshot-multi-node",
		},
	}, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
		{ControlPlane: false, ETCD: false, Worker: true, Quantity: 1},
	})
	assert.Len(t, fixture.pods, 2)

	podNames := []string{fixture.pods[0].Name, fixture.pods[1].Name}
	expectedNodeByPod := map[string]string{
		fixture.pods[0].Name: "imported-init-0",
		fixture.pods[1].Name: "imported-node-1",
	}

	waitForImportedNodesReady(t, clients, fixture.ns.Name, fixture.pods[0].Name, fixture.kubectlEnv, []string{"imported-init-0", "imported-node-1"})

	phase1Cluster := waitForClusterAnnotationValue(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "true", importedIdentityWaitTimeout)
	assert.Equal(t, "true", phase1Cluster.Annotations[opsEnabledAnnotation])

	waitForDownstreamSystemAgentPlanDeleteMode(t, clients, fixture.ns.Name, fixture.pods[0].Name, fixture.kubectlEnv, "false", importedIdentityWaitTimeout)

	waitForSystemAgentActiveStateOnPods(t, clients, fixture.ns.Name, podNames, true, importedSystemAgentTransitionTimeout)
	phase1SecretSet := waitForPodsConnectionInfoToMatchNodePlanIdentity(t, clients, fixture.mgmtCluster.Name, fixture.ns.Name, expectedNodeByPod, importedIdentityWaitTimeout)
	phase1Identity := waitForImportedPlanIdentity(t, clients, fixture.mgmtCluster.Name, len(phase1SecretSet), true, importedIdentityWaitTimeout)
	assert.Len(t, phase1Identity.MachinePlanSecrets, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanServiceAccount, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanTokenSecrets, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanRoles, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanRoleBindings, len(phase1SecretSet))
	assert.Equal(t, len(expectedNodeByPod), len(phase1SecretSet), "each imported node should have its own machine-plan identity after enable")

	waitForAppliedSystemAgentHash(t, clients, fixture.mgmtCluster.Name, importedIdentityWaitTimeout)

	if _, err := setClusterAnnotation(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "false"); err != nil {
		t.Fatal(err)
	}

	phase2Cluster := waitForImportedDay2OpsDisabled(t, clients, fixture.mgmtCluster.Name, importedDisableWaitTimeout)
	assert.Equal(t, "false", phase2Cluster.Annotations[opsEnabledAnnotation])
	assert.Empty(t, phase2Cluster.Annotations[importedCleaningStateAnnotation])
	assert.Empty(t, phase2Cluster.Annotations[appliedSystemAgentHashAnnotation])

	phase2Identity := waitForImportedPlanIdentity(t, clients, fixture.mgmtCluster.Name, 0, true, importedDisableWaitTimeout)
	assert.Len(t, phase2Identity.MachinePlanSecrets, 0)
	assert.Len(t, phase2Identity.PlanServiceAccount, 0)
	assert.Len(t, phase2Identity.PlanTokenSecrets, 0)
	assert.Len(t, phase2Identity.PlanRoles, 0)
	assert.Len(t, phase2Identity.PlanRoleBindings, 0)

	waitForSystemAgentActiveStateOnPods(t, clients, fixture.ns.Name, podNames, false, importedSystemAgentTransitionTimeout)

	if _, err := setClusterAnnotation(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "true"); err != nil {
		t.Fatal(err)
	}

	phase3Cluster := waitForClusterAnnotationValue(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "true", importedIdentityWaitTimeout)
	assert.Equal(t, "true", phase3Cluster.Annotations[opsEnabledAnnotation])
	assert.NotEmpty(t, waitForAppliedSystemAgentHash(t, clients, fixture.mgmtCluster.Name, importedIdentityWaitTimeout))

	waitForDownstreamSystemAgentPlanDeleteMode(t, clients, fixture.ns.Name, fixture.pods[0].Name, fixture.kubectlEnv, "false", importedIdentityWaitTimeout)

	waitForSystemAgentActiveStateOnPods(t, clients, fixture.ns.Name, podNames, true, importedSystemAgentTransitionTimeout)
	phase3SecretSet := waitForPodsConnectionInfoToMatchNodePlanIdentity(t, clients, fixture.mgmtCluster.Name, fixture.ns.Name, expectedNodeByPod, importedIdentityWaitTimeout)
	phase3Identity := waitForImportedPlanIdentity(t, clients, fixture.mgmtCluster.Name, len(phase3SecretSet), true, importedIdentityWaitTimeout)
	assert.Len(t, phase3Identity.MachinePlanSecrets, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanServiceAccount, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanTokenSecrets, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanRoles, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanRoleBindings, len(phase3SecretSet))
	assert.Equal(t, len(expectedNodeByPod), len(phase3SecretSet), "each imported node should have its own machine-plan identity after re-enable")
	assert.False(t, sameSecretNameSet(phase1SecretSet, phase3SecretSet), "re-enabled machine-plan identity should be recreated with fresh secret names")

	etcdMachinePlanSecretName := findEtcdControlPlaneMachinePlanSecretName(t, phase3Identity)
	preSaveFeedback := getMachinePlanFeedbackState(t, clients, fixture.mgmtCluster.Name, etcdMachinePlanSecretName)
	saveOp := RunETCDSnapshotSaveOperationTest(t, clients, fixture.ns.Name, fixture.clusterRef)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, saveOp.Status.Phase)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&saveOp.Status))
	assert.NotContains(t, opv1alpha1.InProgressCondition.GetMessage(&saveOp.Status), planner.WaitingPlanStatusMessage)

	waitForMachinePlanFeedbackAfterOperation(
		t,
		clients,
		fixture.mgmtCluster.Name,
		etcdMachinePlanSecretName,
		preSaveFeedback,
		saveOp.CreationTimestamp.Time,
		importedIdentityWaitTimeout,
	)
}

func waitForSystemAgentActiveStateOnPods(t *testing.T, clients *clients.Clients, namespace string, podNames []string, expectedActive bool, timeout time.Duration) {
	t.Helper()

	for i := range podNames {
		waitForSystemAgentActiveState(t, clients, namespace, podNames[i], expectedActive, timeout)
	}
}

func waitForPodsConnectionInfoToMatchNodePlanIdentity(
	t *testing.T,
	clients *clients.Clients,
	clusterName, namespace string,
	expectedNodeByPod map[string]string,
	timeout time.Duration,
) map[string]struct{} {
	t.Helper()

	var (
		lastByPod     = map[string]string{}
		lastSecretSet = map[string]struct{}{}
		lastErr       error
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		current := map[string]string{}
		usedSecrets := map[string]struct{}{}
		for podName, expectedNodeName := range expectedNodeByPod {
			secretName, err := readConnectionInfoSecretName(clients, namespace, podName)
			if err != nil {
				lastErr = err
				return false, nil
			}
			secret, err := clients.Core.Secret().Get(clusterName, secretName, metav1.GetOptions{})
			if err != nil {
				lastErr = err
				return false, nil
			}
			if secret.Labels[capr.NodeNameLabel] != expectedNodeName {
				lastErr = fmt.Errorf(
					"pod %s expected node %q but connected to secret %s/%s with node label %q",
					podName,
					expectedNodeName,
					secret.Namespace,
					secret.Name,
					secret.Labels[capr.NodeNameLabel],
				)
				return false, nil
			}

			current[podName] = secretName
			usedSecrets[secretName] = struct{}{}
		}
		lastByPod = current
		lastSecretSet = usedSecrets
		return len(usedSecrets) == len(expectedNodeByPod), nil
	})
	if err != nil {
		t.Fatalf(
			"timed out waiting for pod connection info to match node plan identity mapping: %v (lastByPod=%v lastSecretCount=%d lastErr=%v)",
			err,
			lastByPod,
			len(lastSecretSet),
			lastErr,
		)
	}
	return lastSecretSet
}

func machinePlanSecretNameSet(identity importedPlanIdentity) map[string]struct{} {
	result := make(map[string]struct{}, len(identity.MachinePlanSecrets))
	for i := range identity.MachinePlanSecrets {
		result[identity.MachinePlanSecrets[i].Name] = struct{}{}
	}
	return result
}

func sameSecretNameSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func findEtcdControlPlaneMachinePlanSecretName(t *testing.T, identity importedPlanIdentity) string {
	t.Helper()

	for i := range identity.MachinePlanSecrets {
		s := identity.MachinePlanSecrets[i]
		if s.Labels[capr.EtcdRoleLabel] == "true" && s.Labels[capr.ControlPlaneRoleLabel] == "true" {
			return s.Name
		}
	}
	t.Fatal("did not find etcd/control-plane machine-plan secret in recreated identity set")
	return ""
}
