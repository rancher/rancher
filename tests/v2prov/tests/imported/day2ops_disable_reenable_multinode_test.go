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
	"github.com/rancher/rancher/tests/v2prov/defaults"
	"github.com/rancher/rancher/tests/v2prov/namespace"
	"github.com/rancher/rancher/tests/v2prov/registry"
	"github.com/rancher/rancher/tests/v2prov/wait"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

// Test_Operation_SetD_ImportedDay2OpsDisableReenableSnapshotSave_MultiNode validates imported
// day2ops disable/re-enable lifecycle and ETCDSnapshotSave on a two-node imported RKE2 topology.
func Test_Operation_SetD_ImportedDay2OpsDisableReenableSnapshotSave_MultiNode(t *testing.T) {
	clients, err := clients.New()
	if err != nil {
		t.Fatal(err)
	}
	defer clients.Close()

	assertImportedDay2OpsFeatureEnabled(t, clients)

	ns, err := namespace.Random(clients)
	if err != nil {
		t.Fatal(err)
	}

	registryCACert, err := registry.EnsureRegistryCache(clients)
	if err != nil {
		t.Fatal(err)
	}

	pods, err := cluster.NewImportedClusterPods(clients, ns.Name, defaults.SomeK8sVersion, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
		{ControlPlane: false, ETCD: false, Worker: true, Quantity: 1},
	}, nil, registryCACert)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, pods, 2)

	podNames := []string{pods[0].Name, pods[1].Name}
	expectedNodeByPod := map[string]string{
		pods[0].Name: "imported-init-0",
		pods[1].Name: "imported-node-1",
	}

	mgmtCluster, err := cluster.NewImported(clients, &v3.Cluster{
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
	})
	if err != nil {
		t.Fatal(err)
	}

	importCmd, err := cluster.ImportCommand(clients, mgmtCluster)
	handleError(t, clients, mgmtCluster.Name, err)
	assert.NotEmpty(t, importCmd)

	distro := capr.GetRuntime(defaults.SomeK8sVersion)
	kubeconfig := fmt.Sprintf("/etc/rancher/%s/%s.yaml", distro, distro)
	binDir := fmt.Sprintf("/var/lib/rancher/%s/bin", distro)
	kubectlEnv := fmt.Sprintf("KUBECONFIG=%s PATH=$PATH:%s", kubeconfig, binDir)

	waitForImportedAPIServer(t, clients, ns.Name, pods[0].Name, kubectlEnv)

	out, err := cluster.ExecOnPod(clients, ns.Name, pods[0].Name, "sh", "-c", fmt.Sprintf("export %s && %s", kubectlEnv, importCmd))
	if err != nil {
		t.Fatalf("import command failed: %v\noutput: %s", err, out)
	}

	err = wait.ClusterObject(clients.Ctx, clients.Mgmt.Cluster().Watch, mgmtCluster, func(obj runtime.Object) (bool, error) {
		mgmtCluster = obj.(*v3.Cluster)
		return v3.Ready.IsTrue(mgmtCluster), nil
	})
	handleError(t, clients, mgmtCluster.Name, err)

	waitForImportedNodesReady(t, clients, ns.Name, pods[0].Name, kubectlEnv, []string{"imported-init-0", "imported-node-1"})

	clusterRef := corev1.ObjectReference{
		APIVersion: "management.cattle.io/v3",
		Kind:       "Cluster",
		Name:       mgmtCluster.Name,
	}

	phase1Cluster := waitForClusterAnnotationValue(t, clients, mgmtCluster.Name, opsEnabledAnnotation, "true", importedIdentityWaitTimeout)
	assert.Equal(t, "true", phase1Cluster.Annotations[opsEnabledAnnotation])

	waitForDownstreamSystemAgentPlanDeleteMode(t, clients, ns.Name, pods[0].Name, kubectlEnv, "false", importedIdentityWaitTimeout)

	waitForSystemAgentActiveStateOnPods(t, clients, ns.Name, podNames, true, importedSystemAgentTransitionTimeout)
	phase1SecretSet := waitForPodsConnectionInfoToMatchNodePlanIdentity(t, clients, mgmtCluster.Name, ns.Name, expectedNodeByPod, importedIdentityWaitTimeout)
	phase1Identity := waitForImportedPlanIdentity(t, clients, mgmtCluster.Name, len(phase1SecretSet), true, importedIdentityWaitTimeout)
	assert.Len(t, phase1Identity.MachinePlanSecrets, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanServiceAccount, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanTokenSecrets, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanRoles, len(phase1SecretSet))
	assert.Len(t, phase1Identity.PlanRoleBindings, len(phase1SecretSet))
	assert.Equal(t, len(expectedNodeByPod), len(phase1SecretSet), "each imported node should have its own machine-plan identity after enable")

	waitForAppliedSystemAgentHash(t, clients, mgmtCluster.Name, importedIdentityWaitTimeout)

	if _, err := setClusterAnnotation(t, clients, mgmtCluster.Name, opsEnabledAnnotation, "false"); err != nil {
		t.Fatal(err)
	}

	phase2Cluster := waitForImportedResetComplete(t, clients, mgmtCluster.Name, importedResetWaitTimeout)
	assert.Equal(t, "false", phase2Cluster.Annotations[opsEnabledAnnotation])
	assert.Empty(t, phase2Cluster.Annotations[importedCleaningStateAnnotation])
	assert.Empty(t, phase2Cluster.Annotations[appliedSystemAgentHashAnnotation])

	phase2Identity := waitForImportedPlanIdentity(t, clients, mgmtCluster.Name, 0, true, importedResetWaitTimeout)
	assert.Len(t, phase2Identity.MachinePlanSecrets, 0)
	assert.Len(t, phase2Identity.PlanServiceAccount, 0)
	assert.Len(t, phase2Identity.PlanTokenSecrets, 0)
	assert.Len(t, phase2Identity.PlanRoles, 0)
	assert.Len(t, phase2Identity.PlanRoleBindings, 0)

	waitForSystemAgentActiveStateOnPods(t, clients, ns.Name, podNames, false, importedSystemAgentTransitionTimeout)

	if _, err := setClusterAnnotation(t, clients, mgmtCluster.Name, opsEnabledAnnotation, "true"); err != nil {
		t.Fatal(err)
	}

	phase3Cluster := waitForClusterAnnotationValue(t, clients, mgmtCluster.Name, opsEnabledAnnotation, "true", importedIdentityWaitTimeout)
	assert.Equal(t, "true", phase3Cluster.Annotations[opsEnabledAnnotation])
	assert.NotEmpty(t, waitForAppliedSystemAgentHash(t, clients, mgmtCluster.Name, importedIdentityWaitTimeout))

	waitForDownstreamSystemAgentPlanDeleteMode(t, clients, ns.Name, pods[0].Name, kubectlEnv, "false", importedIdentityWaitTimeout)

	waitForSystemAgentActiveStateOnPods(t, clients, ns.Name, podNames, true, importedSystemAgentTransitionTimeout)
	phase3SecretSet := waitForPodsConnectionInfoToMatchNodePlanIdentity(t, clients, mgmtCluster.Name, ns.Name, expectedNodeByPod, importedIdentityWaitTimeout)
	phase3Identity := waitForImportedPlanIdentity(t, clients, mgmtCluster.Name, len(phase3SecretSet), true, importedIdentityWaitTimeout)
	assert.Len(t, phase3Identity.MachinePlanSecrets, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanServiceAccount, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanTokenSecrets, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanRoles, len(phase3SecretSet))
	assert.Len(t, phase3Identity.PlanRoleBindings, len(phase3SecretSet))
	assert.Equal(t, len(expectedNodeByPod), len(phase3SecretSet), "each imported node should have its own machine-plan identity after re-enable")
	assert.False(t, sameSecretNameSet(phase1SecretSet, phase3SecretSet), "re-enabled machine-plan identity should be recreated with fresh secret names")

	etcdMachinePlanSecretName := findEtcdControlPlaneMachinePlanSecretName(t, phase3Identity)
	preSaveFeedback := getMachinePlanFeedbackState(t, clients, mgmtCluster.Name, etcdMachinePlanSecretName)
	saveOp := RunETCDSnapshotSaveOperationTest(t, clients, ns.Name, clusterRef)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, saveOp.Status.Phase)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&saveOp.Status))
	assert.NotContains(t, opv1alpha1.InProgressCondition.GetMessage(&saveOp.Status), planner.WaitingPlanStatusMessage)

	waitForMachinePlanFeedbackAfterOperation(
		t,
		clients,
		mgmtCluster.Name,
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
