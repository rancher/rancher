package imported

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	v3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	"github.com/rancher/rancher/pkg/capr/planner"
	"github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/tests/v2prov/clients"
	"github.com/rancher/rancher/tests/v2prov/cluster"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const (
	opsEnabledAnnotation                 = "operations.cattle.io/ops-enabled"
	importedCleaningStateAnnotation      = "operations.cattle.io/imported-cleaning-state"
	appliedSystemAgentHashAnnotation     = "management.cattle.io/applied-system-agent-upgrader-hash"
	systemAgentConnectionInfoPath        = "/var/lib/rancher/agent/rancher2_connection_info.json"
	importedDay2OpsFeatureName           = "imported-day-2-ops"
	importedNodeAPIServerWaitTimeout     = 5 * time.Minute
	importedIdentityWaitTimeout          = 25 * time.Minute
	importedDisableWaitTimeout           = 25 * time.Minute
	importedSystemAgentTransitionTimeout = 25 * time.Minute

	systemAgentUpgraderPlanName = "system-agent-upgrader"
)

type importedPlanIdentity struct {
	MachinePlanSecrets []corev1.Secret
	PlanServiceAccount []corev1.ServiceAccount
	PlanTokenSecrets   []corev1.Secret
	PlanRoles          []rbacv1.Role
	PlanRoleBindings   []rbacv1.RoleBinding
}

type connectionInfo struct {
	SecretName string `json:"secretName"`
}

type machinePlanFeedbackState struct {
	AppliedPlanHash  string
	ProbeStatusesLen int
	PlanLastUpdated  string
	ProbesPassed     string
}

// Test_Imported_Operation_SetD_ImportedDay2OpsDisableReenableSnapshotSave validates the imported day2ops
// disable/re-enable lifecycle and verifies ETCDSnapshotSave still succeeds after re-enable.
func Test_Imported_Operation_SetD_ImportedDay2OpsDisableReenableSnapshotSave(t *testing.T) {
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
			DisplayName:    "test-imported-day2ops-disable-reenable-snapshot",
		},
	}, []cluster.ImportedNodePool{
		{ControlPlane: true, ETCD: true, Worker: true, Quantity: 1},
	})
	assert.Len(t, fixture.pods, 1)
	t.Logf("created imported cluster pod %s/%s", fixture.ns.Name, fixture.pods[0].Name)
	t.Logf("management cluster %s reached Ready", fixture.mgmtCluster.Name)

	phase1Cluster := waitForClusterAnnotationValue(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "true", importedIdentityWaitTimeout)
	assert.Equal(t, "true", phase1Cluster.Annotations[opsEnabledAnnotation])
	t.Logf("phase 1: ops-enabled=true observed on cluster %s", fixture.mgmtCluster.Name)

	phase1Identity := waitForImportedPlanIdentity(t, clients, fixture.mgmtCluster.Name, 1, true, importedIdentityWaitTimeout)
	assert.Len(t, phase1Identity.MachinePlanSecrets, 1)
	assert.Len(t, phase1Identity.PlanServiceAccount, 1)
	assert.Len(t, phase1Identity.PlanTokenSecrets, 1)
	assert.Len(t, phase1Identity.PlanRoles, 1)
	assert.Len(t, phase1Identity.PlanRoleBindings, 1)
	waitForDownstreamSystemAgentPlanUninstallMode(t, clients, fixture.ns.Name, fixture.pods[0].Name, fixture.kubectlEnv, "false", importedIdentityWaitTimeout)
	t.Logf("phase 1 identity: %s", summarizeImportedPlanIdentity(phase1Identity))

	waitForSystemAgentActiveState(t, clients, fixture.ns.Name, fixture.pods[0].Name, true, importedSystemAgentTransitionTimeout)
	t.Logf("phase 1: rancher-system-agent is active on pod %s/%s", fixture.ns.Name, fixture.pods[0].Name)

	initialMachinePlanSecretName := phase1Identity.MachinePlanSecrets[0].Name
	initialMachinePlanSecretUID := phase1Identity.MachinePlanSecrets[0].UID
	initialPlanTokenSecretName := phase1Identity.PlanTokenSecrets[0].Name
	waitForConnectionInfoSecretName(t, clients, fixture.ns.Name, fixture.pods[0].Name, initialMachinePlanSecretName, importedIdentityWaitTimeout)
	t.Logf("phase 1: connection info points to initial machine-plan secret %s", initialMachinePlanSecretName)

	initialHash := waitForAppliedSystemAgentHash(t, clients, fixture.mgmtCluster.Name, importedIdentityWaitTimeout)
	t.Logf("phase 1: applied system-agent hash=%s", initialHash)

	t.Logf("phase 2: disabling imported day2ops on cluster %s", fixture.mgmtCluster.Name)
	if _, err := setClusterAnnotation(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "false"); err != nil {
		t.Fatal(err)
	}
	waitForDownstreamSystemAgentPlanUninstallMode(t, clients, fixture.ns.Name, fixture.pods[0].Name, fixture.kubectlEnv, "true", importedIdentityWaitTimeout)
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
	t.Logf("phase 2 identity: %s", summarizeImportedPlanIdentity(phase2Identity))

	waitForSystemAgentActiveState(t, clients, fixture.ns.Name, fixture.pods[0].Name, false, importedSystemAgentTransitionTimeout)
	t.Logf("phase 2: rancher-system-agent is inactive on pod %s/%s", fixture.ns.Name, fixture.pods[0].Name)

	t.Logf("phase 3: re-enabling imported day2ops on cluster %s", fixture.mgmtCluster.Name)
	if _, err := setClusterAnnotation(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "true"); err != nil {
		t.Fatal(err)
	}

	phase3Cluster := waitForClusterAnnotationValue(t, clients, fixture.mgmtCluster.Name, opsEnabledAnnotation, "true", importedIdentityWaitTimeout)
	assert.Equal(t, "true", phase3Cluster.Annotations[opsEnabledAnnotation])
	phase3Hash := waitForAppliedSystemAgentHash(t, clients, fixture.mgmtCluster.Name, importedIdentityWaitTimeout)
	assert.NotEmpty(t, phase3Hash)
	t.Logf("phase 3: applied system-agent hash=%s", phase3Hash)

	phase3Identity := waitForImportedPlanIdentity(t, clients, fixture.mgmtCluster.Name, 1, true, importedIdentityWaitTimeout)
	assert.Len(t, phase3Identity.MachinePlanSecrets, 1)
	assert.Len(t, phase3Identity.PlanServiceAccount, 1)
	assert.Len(t, phase3Identity.PlanTokenSecrets, 1)
	assert.Len(t, phase3Identity.PlanRoles, 1)
	assert.Len(t, phase3Identity.PlanRoleBindings, 1)
	waitForDownstreamSystemAgentPlanUninstallMode(t, clients, fixture.ns.Name, fixture.pods[0].Name, fixture.kubectlEnv, "false", importedIdentityWaitTimeout)
	t.Logf("phase 3 identity: %s", summarizeImportedPlanIdentity(phase3Identity))

	newMachinePlanSecretName := phase3Identity.MachinePlanSecrets[0].Name
	newMachinePlanSecretUID := phase3Identity.MachinePlanSecrets[0].UID
	newPlanTokenSecretName := phase3Identity.PlanTokenSecrets[0].Name
	assert.NotEqual(t, initialMachinePlanSecretName, newMachinePlanSecretName)
	assert.NotEqual(t, initialMachinePlanSecretUID, newMachinePlanSecretUID)
	assert.NotEqual(t, initialPlanTokenSecretName, newPlanTokenSecretName)
	t.Logf(
		"phase 3: imported plan identity was recreated with new machine-plan secret %s (was %s), new uid %s (was %s), and token changed from %s to %s",
		newMachinePlanSecretName,
		initialMachinePlanSecretName,
		newMachinePlanSecretUID,
		initialMachinePlanSecretUID,
		initialPlanTokenSecretName,
		newPlanTokenSecretName,
	)

	waitForSystemAgentActiveState(t, clients, fixture.ns.Name, fixture.pods[0].Name, true, importedSystemAgentTransitionTimeout)
	waitForConnectionInfoSecretName(t, clients, fixture.ns.Name, fixture.pods[0].Name, newMachinePlanSecretName, importedIdentityWaitTimeout)
	t.Logf("phase 3: rancher-system-agent is active and connection info points to %s", newMachinePlanSecretName)

	preSaveFeedback := getMachinePlanFeedbackState(t, clients, fixture.mgmtCluster.Name, newMachinePlanSecretName)
	t.Logf("pre-save machine-plan feedback on %s: %s", newMachinePlanSecretName, formatMachinePlanFeedbackState(preSaveFeedback))
	saveOp := RunETCDSnapshotSaveOperationTest(t, clients, fixture.ns.Name, fixture.clusterRef)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, saveOp.Status.Phase)
	assert.NotEqual(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&saveOp.Status))
	assert.NotContains(t, opv1alpha1.InProgressCondition.GetMessage(&saveOp.Status), planner.WaitingPlanStatusMessage)
	t.Logf("snapshot save operation %s/%s completed with phase=%s", saveOp.Namespace, saveOp.Name, saveOp.Status.Phase)

	waitForMachinePlanFeedbackAfterOperation(
		t,
		clients,
		fixture.mgmtCluster.Name,
		newMachinePlanSecretName,
		preSaveFeedback,
		saveOp.CreationTimestamp.Time,
		importedIdentityWaitTimeout,
	)
	t.Logf("post-save machine-plan feedback on %s advanced after snapshot save", newMachinePlanSecretName)
}

func assertImportedDay2OpsFeatureEnabled(t *testing.T, clients *clients.Clients) {
	t.Helper()

	feature, err := clients.Mgmt.Feature().Get(importedDay2OpsFeatureName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	enabled := feature.Status.Default
	if feature.Spec.Value != nil {
		enabled = *feature.Spec.Value
	}
	if !enabled {
		t.Skip("imported day2ops feature flag is disabled")
	}
}

func waitForImportedAPIServer(t *testing.T, clients *clients.Clients, namespace, podName, kubectlEnv string) {
	t.Helper()

	var (
		lastErr error
		lastOut string
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, importedNodeAPIServerWaitTimeout, true, func(_ context.Context) (bool, error) {
		lastOut, lastErr = cluster.ExecOnPod(clients, namespace, podName, "sh", "-c", fmt.Sprintf("export %s && kubectl get nodes", kubectlEnv))
		return lastErr == nil, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for downstream API server to be ready: %v (lastErr=%v output=%q)", err, lastErr, strings.TrimSpace(lastOut))
	}
}

func waitForDownstreamSystemAgentPlanUninstallMode(
	t *testing.T,
	clients *clients.Clients,
	namespace, podName, kubectlEnv, expectedUninstall string,
	timeout time.Duration,
) {
	t.Helper()

	var (
		lastOutput         string
		lastErr            error
		lastUninstallValue string
		lastUninstallCount int
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		lastOutput, lastErr = cluster.ExecOnPod(
			clients,
			namespace,
			podName,
			"sh",
			"-c",
			fmt.Sprintf(
				"export %s && kubectl -n cattle-system get plans.upgrade.cattle.io %s --ignore-not-found -o jsonpath='{range .spec.upgrade.envs[*]}{.name}={.value}{\"\\n\"}{end}'",
				kubectlEnv,
				systemAgentUpgraderPlanName,
			),
		)
		if lastErr != nil {
			return false, nil
		}

		lastUninstallCount = 0
		lastUninstallValue = ""
		for _, line := range strings.Split(strings.TrimSpace(lastOutput), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "UNINSTALL=") {
				continue
			}
			lastUninstallCount++
			lastUninstallValue = strings.TrimSpace(strings.TrimPrefix(line, "UNINSTALL="))
		}

		return lastUninstallCount == 1 && lastUninstallValue == expectedUninstall, nil
	})
	if err != nil {
		t.Fatalf(
			"timed out waiting for downstream plan %s UNINSTALL=%s exactly once on pod %s/%s: %v (lastUninstallCount=%d lastUninstallValue=%q lastErr=%v output=%q)",
			systemAgentUpgraderPlanName,
			expectedUninstall,
			namespace,
			podName,
			err,
			lastUninstallCount,
			lastUninstallValue,
			lastErr,
			strings.TrimSpace(lastOutput),
		)
	}
}

func waitForClusterAnnotationValue(t *testing.T, clients *clients.Clients, clusterName, annotation, expected string, timeout time.Duration) *v3.Cluster {
	t.Helper()

	var clusterObj *v3.Cluster
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		var err error
		clusterObj, err = clients.Mgmt.Cluster().Get(clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return clusterObj.Annotations[annotation] == expected, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for cluster %s annotation %s=%s: %v", clusterName, annotation, expected, err)
	}
	return clusterObj
}

func waitForAppliedSystemAgentHash(t *testing.T, clients *clients.Clients, clusterName string, timeout time.Duration) string {
	t.Helper()

	var (
		clusterObj *v3.Cluster
		hash       string
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		var err error
		clusterObj, err = clients.Mgmt.Cluster().Get(clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		hash = clusterObj.Annotations[appliedSystemAgentHashAnnotation]
		return hash != "", nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for cluster %s applied system-agent hash: %v", clusterName, err)
	}
	return hash
}

func waitForImportedDay2OpsDisabled(t *testing.T, clients *clients.Clients, clusterName string, timeout time.Duration) *v3.Cluster {
	t.Helper()

	var clusterObj *v3.Cluster
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		var err error
		clusterObj, err = clients.Mgmt.Cluster().Get(clusterName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if clusterObj.Annotations[opsEnabledAnnotation] != "false" {
			return false, nil
		}
		if clusterObj.Annotations[importedCleaningStateAnnotation] != "" {
			return false, nil
		}
		if clusterObj.Annotations[appliedSystemAgentHashAnnotation] != "" {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for imported day2ops to be disabled on cluster %s: %v", clusterName, err)
	}
	return clusterObj
}

func setClusterAnnotation(t *testing.T, clients *clients.Clients, clusterName, key, value string) (*v3.Cluster, error) {
	t.Helper()

	var updated *v3.Cluster
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := clients.Mgmt.Cluster().Get(clusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		current = current.DeepCopy()
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations[key] = value
		updated, err = clients.Mgmt.Cluster().Update(current)
		return err
	})
	return updated, err
}

func waitForImportedPlanIdentity(t *testing.T, clients *clients.Clients, clusterName string, expectedCount int, assertTokenSecrets bool, timeout time.Duration) importedPlanIdentity {
	t.Helper()

	var identity importedPlanIdentity
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		current, err := collectImportedPlanIdentity(clients, clusterName)
		if err != nil {
			return false, err
		}
		identity = current

		if len(identity.MachinePlanSecrets) != expectedCount {
			return false, nil
		}
		if len(identity.PlanServiceAccount) != expectedCount {
			return false, nil
		}
		if len(identity.PlanRoles) != expectedCount {
			return false, nil
		}
		if len(identity.PlanRoleBindings) != expectedCount {
			return false, nil
		}
		if assertTokenSecrets && len(identity.PlanTokenSecrets) != expectedCount {
			return false, nil
		}
		return importedPlanIdentityHasExpectedShape(identity, assertTokenSecrets), nil
	})
	if err != nil {
		t.Fatalf(
			"timed out waiting for imported plan identity count=%d (token=%t): %v (got machinePlan=%d sa=%d token=%d role=%d roleBinding=%d details=%s)",
			expectedCount,
			assertTokenSecrets,
			err,
			len(identity.MachinePlanSecrets),
			len(identity.PlanServiceAccount),
			len(identity.PlanTokenSecrets),
			len(identity.PlanRoles),
			len(identity.PlanRoleBindings),
			summarizeImportedPlanIdentity(identity),
		)
	}
	return identity
}

func collectImportedPlanIdentity(clients *clients.Clients, clusterName string) (importedPlanIdentity, error) {
	identity := importedPlanIdentity{}

	machinePlans, err := clients.Core.Secret().List(clusterName, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", capr.ClusterNameLabel, clusterName),
		FieldSelector: fmt.Sprintf("type=%s", capr.SecretTypeMachinePlan),
	})
	if err != nil {
		return identity, err
	}
	identity.MachinePlanSecrets = machinePlans.Items

	serviceAccounts, err := clients.Core.ServiceAccount().List(clusterName, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", capr.ClusterNameLabel, clusterName, capr.RoleLabel, capr.RolePlan),
	})
	if err != nil {
		return identity, err
	}
	identity.PlanServiceAccount = serviceAccounts.Items

	for i := range identity.PlanServiceAccount {
		name := identity.PlanServiceAccount[i].Name

		role, err := clients.RBAC.Role().Get(clusterName, name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return identity, err
			}
		} else {
			identity.PlanRoles = append(identity.PlanRoles, *role)
		}

		roleBinding, err := clients.RBAC.RoleBinding().Get(clusterName, name, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return identity, err
			}
		} else {
			identity.PlanRoleBindings = append(identity.PlanRoleBindings, *roleBinding)
		}
	}

	tokenSecrets, err := clients.Core.Secret().List(clusterName, metav1.ListOptions{
		LabelSelector: serviceaccounttoken.ServiceAccountSecretLabel,
		FieldSelector: fmt.Sprintf("type=%s", corev1.SecretTypeServiceAccountToken),
	})
	if err != nil {
		return identity, err
	}
	for i := range tokenSecrets.Items {
		saName := tokenSecrets.Items[i].Labels[serviceaccounttoken.ServiceAccountSecretLabel]
		if strings.HasSuffix(saName, "-machine-plan") {
			identity.PlanTokenSecrets = append(identity.PlanTokenSecrets, tokenSecrets.Items[i])
		}
	}

	return identity, nil
}

func importedPlanIdentityHasExpectedShape(identity importedPlanIdentity, assertTokenSecrets bool) bool {
	if len(identity.MachinePlanSecrets) != len(identity.PlanServiceAccount) {
		return false
	}
	if len(identity.PlanServiceAccount) != len(identity.PlanRoles) {
		return false
	}
	if len(identity.PlanServiceAccount) != len(identity.PlanRoleBindings) {
		return false
	}

	serviceAccounts := make(map[string]struct{}, len(identity.PlanServiceAccount))
	for i := range identity.PlanServiceAccount {
		serviceAccounts[identity.PlanServiceAccount[i].Name] = struct{}{}
	}
	for i := range identity.MachinePlanSecrets {
		if _, ok := serviceAccounts[identity.MachinePlanSecrets[i].Name]; !ok {
			return false
		}
	}
	for i := range identity.PlanRoles {
		if _, ok := serviceAccounts[identity.PlanRoles[i].Name]; !ok {
			return false
		}
	}
	for i := range identity.PlanRoleBindings {
		if _, ok := serviceAccounts[identity.PlanRoleBindings[i].Name]; !ok {
			return false
		}
	}

	if !assertTokenSecrets {
		return true
	}
	tokenByServiceAccount := map[string]int{}
	for i := range identity.PlanTokenSecrets {
		saName := identity.PlanTokenSecrets[i].Labels[serviceaccounttoken.ServiceAccountSecretLabel]
		tokenByServiceAccount[saName]++
	}
	for sa := range serviceAccounts {
		if tokenByServiceAccount[sa] != 1 {
			return false
		}
	}
	return true
}

func waitForSystemAgentActiveState(t *testing.T, clients *clients.Clients, namespace, podName string, expectedActive bool, timeout time.Duration) {
	t.Helper()

	var (
		lastState string
		lastErr   error
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		lastState, lastErr = systemAgentState(clients, namespace, podName)
		if lastErr != nil {
			return false, nil
		}
		if expectedActive {
			return lastState == "active", nil
		}
		return lastState != "active", nil
	})
	if err != nil {
		diagnostics := collectSystemAgentDiagnostics(clients, namespace, podName)
		t.Fatalf(
			"timed out waiting for rancher-system-agent active=%t on pod %s/%s: %v (lastState=%q lastErr=%v diagnostics=%q)",
			expectedActive,
			namespace,
			podName,
			err,
			lastState,
			lastErr,
			diagnostics,
		)
	}
}

func systemAgentState(clients *clients.Clients, namespace, podName string) (string, error) {
	out, err := cluster.ExecOnPod(clients, namespace, podName, "sh", "-c", "systemctl is-active rancher-system-agent 2>/dev/null || true")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func collectSystemAgentDiagnostics(clients *clients.Clients, namespace, podName string) string {
	connectionSecretName, err := readConnectionInfoSecretName(clients, namespace, podName)
	if err != nil {
		connectionSecretName = fmt.Sprintf("error:%v", err)
	}

	out, cmdErr := cluster.ExecOnPod(clients, namespace, podName, "sh", "-c", `
echo '[status]'
systemctl status rancher-system-agent --no-pager 2>/dev/null | sed -n '1,12p' || true
echo '[enabled]'
systemctl is-enabled rancher-system-agent 2>/dev/null || true
echo '[files]'
ls -l /usr/local/bin/rancher-system-agent /usr/local/bin/rancher-system-agent-uninstall.sh 2>/dev/null || true
ls -l /etc/systemd/system/rancher-system-agent.service /etc/systemd/system/rancher-system-agent.env 2>/dev/null || true
echo '[journal]'
journalctl -u rancher-system-agent -n 40 --no-pager 2>/dev/null | grep -E 'uninstall|delete|remove|stop|start|signal|Failed|Started|Stopping|Deactivated' || true
`)
	if cmdErr != nil {
		return fmt.Sprintf("connectionSecretName=%s diagErr=%v", connectionSecretName, cmdErr)
	}

	return fmt.Sprintf("connectionSecretName=%s %s", connectionSecretName, strings.TrimSpace(out))
}

func waitForConnectionInfoSecretName(t *testing.T, clients *clients.Clients, namespace, podName, expectedSecretName string, timeout time.Duration) {
	t.Helper()

	var (
		lastSecret string
		lastErr    error
	)
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		lastSecret, lastErr = readConnectionInfoSecretName(clients, namespace, podName)
		if lastErr != nil {
			return false, nil
		}
		return lastSecret == expectedSecretName, nil
	})
	if err != nil {
		t.Fatalf(
			"timed out waiting for connection info secretName=%s on pod %s/%s: %v (lastSecret=%q lastErr=%v)",
			expectedSecretName,
			namespace,
			podName,
			err,
			lastSecret,
			lastErr,
		)
	}
}

func readConnectionInfoSecretName(clients *clients.Clients, namespace, podName string) (string, error) {
	out, err := cluster.ExecOnPod(
		clients,
		namespace,
		podName,
		"sh",
		"-c",
		fmt.Sprintf("cat %s", systemAgentConnectionInfoPath),
	)
	if err != nil {
		return "", err
	}

	var info connectionInfo
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		return "", err
	}
	if info.SecretName == "" {
		return "", fmt.Errorf("connection info secretName is empty")
	}
	return info.SecretName, nil
}

func getMachinePlanFeedbackState(t *testing.T, clients *clients.Clients, clusterName, secretName string) machinePlanFeedbackState {
	t.Helper()

	secret, err := clients.Core.Secret().Get(clusterName, secretName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}

	return machinePlanFeedbackState{
		AppliedPlanHash:  hashBytes(secret.Data["appliedPlan"]),
		ProbeStatusesLen: len(secret.Data["probe-statuses"]),
		PlanLastUpdated:  strings.TrimSpace(secret.Annotations[plan.PlanLastUpdatedAnnotation]),
		ProbesPassed:     strings.TrimSpace(secret.Annotations[plan.PlanProbesPassedAnnotation]),
	}
}

func summarizeImportedPlanIdentity(identity importedPlanIdentity) string {
	return fmt.Sprintf(
		"machinePlan=%v sa=%v token=%v role=%v roleBinding=%v",
		secretNames(identity.MachinePlanSecrets),
		serviceAccountNames(identity.PlanServiceAccount),
		secretNames(identity.PlanTokenSecrets),
		roleNames(identity.PlanRoles),
		roleBindingNames(identity.PlanRoleBindings),
	)
}

func formatMachinePlanFeedbackState(state machinePlanFeedbackState) string {
	return fmt.Sprintf(
		"appliedPlanHash=%q probeStatusesLen=%d planLastUpdated=%q probesPassed=%q",
		state.AppliedPlanHash,
		state.ProbeStatusesLen,
		state.PlanLastUpdated,
		state.ProbesPassed,
	)
}

func secretNames(objs []corev1.Secret) []string {
	names := make([]string, 0, len(objs))
	for i := range objs {
		names = append(names, objs[i].Name)
	}
	return names
}

func serviceAccountNames(objs []corev1.ServiceAccount) []string {
	names := make([]string, 0, len(objs))
	for i := range objs {
		names = append(names, objs[i].Name)
	}
	return names
}

func roleNames(objs []rbacv1.Role) []string {
	names := make([]string, 0, len(objs))
	for i := range objs {
		names = append(names, objs[i].Name)
	}
	return names
}

func roleBindingNames(objs []rbacv1.RoleBinding) []string {
	names := make([]string, 0, len(objs))
	for i := range objs {
		names = append(names, objs[i].Name)
	}
	return names
}

func waitForMachinePlanFeedbackAfterOperation(
	t *testing.T,
	clients *clients.Clients,
	clusterName, secretName string,
	baseline machinePlanFeedbackState,
	operationCreatedAt time.Time,
	timeout time.Duration,
) {
	t.Helper()

	var secret *corev1.Secret
	err := utilwait.PollUntilContextTimeout(clients.Ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		var err error
		secret, err = clients.Core.Secret().Get(clusterName, secretName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if len(secret.Data["appliedPlan"]) == 0 {
			return false, nil
		}
		if len(secret.Data["probe-statuses"]) == 0 {
			return false, nil
		}
		planLastUpdated := strings.TrimSpace(secret.Annotations[plan.PlanLastUpdatedAnnotation])
		if planLastUpdated == "" {
			return false, nil
		}
		probesPassed := strings.TrimSpace(secret.Annotations[plan.PlanProbesPassedAnnotation])
		if probesPassed == "" {
			return false, nil
		}
		lastUpdatedTime, err := time.Parse(time.RFC3339, planLastUpdated)
		if err != nil {
			return false, fmt.Errorf("failed to parse %s=%q: %w", plan.PlanLastUpdatedAnnotation, planLastUpdated, err)
		}
		probesPassedTime, err := time.Parse(time.RFC3339, probesPassed)
		if err != nil {
			return false, fmt.Errorf("failed to parse %s=%q: %w", plan.PlanProbesPassedAnnotation, probesPassed, err)
		}
		if lastUpdatedTime.Before(operationCreatedAt) {
			return false, nil
		}
		if probesPassedTime.Before(operationCreatedAt) {
			return false, nil
		}
		return hashBytes(secret.Data["appliedPlan"]) != baseline.AppliedPlanHash, nil
	})
	if err != nil {
		appliedPlanLen := 0
		probeStatusesLen := 0
		planLastUpdated := ""
		probesPassed := ""
		appliedPlanHash := ""
		if secret != nil {
			appliedPlanLen = len(secret.Data["appliedPlan"])
			probeStatusesLen = len(secret.Data["probe-statuses"])
			planLastUpdated = secret.Annotations[plan.PlanLastUpdatedAnnotation]
			probesPassed = secret.Annotations[plan.PlanProbesPassedAnnotation]
			appliedPlanHash = hashBytes(secret.Data["appliedPlan"])
		}
		t.Fatalf(
			"timed out waiting for fresh machine-plan feedback on %s/%s after snapshot op: %v (baselinePlanHash=%q currentPlanHash=%q appliedPlan=%d probe-statuses=%d plan-last-updated=%q probes-passed=%q)",
			clusterName,
			secretName,
			err,
			baseline.AppliedPlanHash,
			appliedPlanHash,
			appliedPlanLen,
			probeStatusesLen,
			planLastUpdated,
			probesPassed,
		)
	}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
