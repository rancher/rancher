package systemagent

import (
	"strings"
	"testing"
	"time"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/capr"
	mgmtcontrollers "github.com/rancher/rancher/pkg/generated/controllers/management.cattle.io/v3"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	namespaces "github.com/rancher/rancher/pkg/namespace"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/serviceaccounttoken"
	"github.com/rancher/rancher/pkg/types/config"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	rbaccontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/rbac/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func TestDisableBeaconWait(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		beacon   *planv1alpha1.Beacon
		wantWait bool
		wantMsg  string
	}{
		{
			name: "no beacon",
		},
		{
			name: "owned by disable",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{planv1alpha1.BeaconOwnerLabel: disableBeaconOwnerKey},
				},
			},
		},
		{
			name: "owned by another controller",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{planv1alpha1.BeaconOwnerLabel: "etcd-snapshot-save"},
				},
			},
			wantWait: true,
			wantMsg:  "waiting for beacon release from \"etcd-snapshot-save\"",
		},
		{
			name: "active without owner does not block disable",
			beacon: &planv1alpha1.Beacon{
				Status: planv1alpha1.BeaconStatus{Active: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wait, message := disableBeaconWait(tt.beacon)
			if wait != tt.wantWait {
				t.Fatalf("expected wait=%t, got %t", tt.wantWait, wait)
			}
			if message != tt.wantMsg {
				t.Fatalf("expected message %q, got %q", tt.wantMsg, message)
			}
		})
	}
}

func TestShouldReconcileImportedDisable(t *testing.T) {
	t.Parallel()

	if shouldReconcileImportedDisable(nil) {
		t.Fatalf("did not expect disable reconciliation for nil annotations")
	}
	if shouldReconcileImportedDisable(map[string]string{}) {
		t.Fatalf("did not expect disable reconciliation for missing ops-enabled annotation")
	}
	if !shouldReconcileImportedDisable(map[string]string{
		day2OpsEnabledAnnotation: "false",
	}) {
		t.Fatalf("expected explicit disable annotation to trigger reconciliation")
	}
	if !shouldReconcileImportedDisable(map[string]string{
		day2OpsEnabledAnnotation:        "true",
		importedCleaningStateAnnotation: apimgmtv3.ImportedDay2OpsCleaningStateOperations,
	}) {
		t.Fatalf("expected cleaning state to keep disable reconciliation sticky")
	}
}

func TestInstallerSetsUninstallFalseEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "UNINSTALL", Value: "false"},
					{Name: systemAgentUpgraderRunIDEnvName, Value: "inherited-run-id"},
					{Name: "STRICT_VERIFY", Value: "custom-strict"},
				},
			},
		},
	}

	plans := renderedPlans(installer(cluster))
	if len(plans) != 2 {
		t.Fatalf("expected 2 rendered plans, got %d", len(plans))
	}
	for _, plan := range plans {
		value, ok := envVar(plan.Spec.Upgrade.Env, "UNINSTALL")
		if !ok || value != "false" {
			t.Fatalf("expected UNINSTALL=false in install plan %s", plan.Name)
		}
		if len(plan.Spec.Upgrade.Env) == 0 || plan.Spec.Upgrade.Env[0].Name != "UNINSTALL" || plan.Spec.Upgrade.Env[0].Value != "false" {
			t.Fatalf("expected UNINSTALL=false to be the first env in install plan %s", plan.Name)
		}
		uninstallCount := 0
		for _, env := range plan.Spec.Upgrade.Env {
			if env.Name == "UNINSTALL" {
				uninstallCount++
			}
		}
		if uninstallCount != 1 {
			t.Fatalf("expected exactly one UNINSTALL env in install plan %s, got %d", plan.Name, uninstallCount)
		}
		roleNoneValue, ok := envVar(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE")
		if !ok || roleNoneValue != "true" {
			t.Fatalf("expected CATTLE_ROLE_NONE=true in install plan %s", plan.Name)
		}
		if roleNoneCount := envVarCount(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
			t.Fatalf("expected exactly one CATTLE_ROLE_NONE env in install plan %s, got %d", plan.Name, roleNoneCount)
		}
		if strictValue, ok := envVar(plan.Spec.Upgrade.Env, "STRICT_VERIFY"); !ok || strictValue != "custom-strict" {
			t.Fatalf("expected STRICT_VERIFY=custom-strict in install plan %s", plan.Name)
		}
	}
	if !hasPlanName(plans, SystemAgentUpgraderPlanName) {
		t.Fatalf("expected install plan %q to be rendered", SystemAgentUpgraderPlanName)
	}
	if !hasPlanName(plans, SystemAgentUpgraderWindowsPlanName) {
		t.Fatalf("expected install plan %q to be rendered", SystemAgentUpgraderWindowsPlanName)
	}
}

func TestUninstallerSetsUninstallEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "UNINSTALL", Value: "false"},
					{Name: "STRICT_VERIFY", Value: "custom-strict"},
				},
			},
		},
	}

	const rolloutID = "rollout-id-1"
	plans := renderedPlans(uninstaller(cluster, rolloutID))
	if len(plans) != 2 {
		t.Fatalf("expected 2 rendered plans, got %d", len(plans))
	}
	for _, plan := range plans {
		value, ok := envVar(plan.Spec.Upgrade.Env, "UNINSTALL")
		if !ok || value != "true" {
			t.Fatalf("expected UNINSTALL=true in uninstall plan %s", plan.Name)
		}
		if len(plan.Spec.Upgrade.Env) == 0 || plan.Spec.Upgrade.Env[0].Name != "UNINSTALL" || plan.Spec.Upgrade.Env[0].Value != "true" {
			t.Fatalf("expected UNINSTALL=true to be the first env in uninstall plan %s", plan.Name)
		}
		if len(plan.Spec.Upgrade.Env) < 2 || plan.Spec.Upgrade.Env[1].Name != systemAgentUpgraderRunIDEnvName || plan.Spec.Upgrade.Env[1].Value != rolloutID {
			t.Fatalf("expected %s=%s to be the second env in uninstall plan %s", systemAgentUpgraderRunIDEnvName, rolloutID, plan.Name)
		}
		uninstallCount := 0
		runIDCount := 0
		for _, env := range plan.Spec.Upgrade.Env {
			if env.Name == "UNINSTALL" {
				uninstallCount++
			}
			if env.Name == systemAgentUpgraderRunIDEnvName {
				runIDCount++
			}
		}
		if uninstallCount != 1 {
			t.Fatalf("expected exactly one UNINSTALL env in uninstall plan %s, got %d", plan.Name, uninstallCount)
		}
		if runIDCount != 1 {
			t.Fatalf("expected exactly one %s env in uninstall plan %s, got %d", systemAgentUpgraderRunIDEnvName, plan.Name, runIDCount)
		}
		if got := plan.Spec.PostCompleteLabels[systemAgentUpgraderRolloutIDLabel]; got != rolloutID {
			t.Fatalf("expected postCompleteLabels[%s]=%s in uninstall plan %s, got %q", systemAgentUpgraderRolloutIDLabel, rolloutID, plan.Name, got)
		}
		roleNoneValue, ok := envVar(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE")
		if !ok || roleNoneValue != "true" {
			t.Fatalf("expected CATTLE_ROLE_NONE=true in uninstall plan %s", plan.Name)
		}
		if roleNoneCount := envVarCount(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
			t.Fatalf("expected exactly one CATTLE_ROLE_NONE env in uninstall plan %s, got %d", plan.Name, roleNoneCount)
		}
		if strictValue, ok := envVar(plan.Spec.Upgrade.Env, "STRICT_VERIFY"); !ok || strictValue != "custom-strict" {
			t.Fatalf("expected STRICT_VERIFY=custom-strict in uninstall plan %s", plan.Name)
		}
	}
	if !hasPlanName(plans, SystemAgentUpgraderPlanName) {
		t.Fatalf("expected uninstall plan %q to be rendered", SystemAgentUpgraderPlanName)
	}
	if !hasPlanName(plans, SystemAgentUpgraderWindowsPlanName) {
		t.Fatalf("expected uninstall plan %q to be rendered", SystemAgentUpgraderWindowsPlanName)
	}
}

func TestInstallerForcesRoleNoneEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "CATTLE_ROLE_NONE", Value: "false"},
				},
			},
		},
	}

	plans := renderedPlans(installer(cluster))
	if len(plans) != 2 {
		t.Fatalf("expected 2 rendered plans, got %d", len(plans))
	}
	for _, plan := range plans {
		roleNoneValue, ok := envVar(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE")
		if !ok || roleNoneValue != "true" {
			t.Fatalf("expected CATTLE_ROLE_NONE=true in install plan %s", plan.Name)
		}
		if roleNoneCount := envVarCount(plan.Spec.Upgrade.Env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
			t.Fatalf("expected exactly one CATTLE_ROLE_NONE env in install plan %s, got %d", plan.Name, roleNoneCount)
		}
	}
}

func TestUninstallerIncludesSharedSUCObjects(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
	}

	objs := uninstaller(cluster, "rollout-id-1")

	hasServiceAccount := false
	hasClusterRole := false
	hasClusterRoleBinding := false
	for _, obj := range objs {
		switch o := obj.(type) {
		case *corev1.ServiceAccount:
			hasServiceAccount = hasServiceAccount || (o.Name == SystemAgentUpgraderServiceAccountName && o.Namespace == namespaces.System)
		case *rbacv1.ClusterRole:
			hasClusterRole = hasClusterRole || (o.Name == SystemAgentUpgraderClusterRoleName)
		case *rbacv1.ClusterRoleBinding:
			hasClusterRoleBinding = hasClusterRoleBinding || (o.Name == SystemAgentUpgraderClusterRoleBindingName)
		}
	}

	if !hasServiceAccount {
		t.Fatalf("expected uninstaller to include service account %q", SystemAgentUpgraderServiceAccountName)
	}
	if !hasClusterRole {
		t.Fatalf("expected uninstaller to include cluster role %q", SystemAgentUpgraderClusterRoleName)
	}
	if !hasClusterRoleBinding {
		t.Fatalf("expected uninstaller to include cluster role binding %q", SystemAgentUpgraderClusterRoleBindingName)
	}
}

func TestPlanEnvForcesRoleNoneAndPreservesStrictVerify(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "STRICT_VERIFY", Value: "custom-strict"},
					{Name: "CATTLE_ROLE_NONE", Value: "false"},
				},
			},
		},
	}

	env := planEnv(cluster)
	if roleNoneValue, ok := envVar(env, "CATTLE_ROLE_NONE"); !ok || roleNoneValue != "true" {
		t.Fatalf("expected forced CATTLE_ROLE_NONE=true, got %q", roleNoneValue)
	}
	if roleNoneCount := envVarCount(env, "CATTLE_ROLE_NONE"); roleNoneCount != 1 {
		t.Fatalf("expected exactly one CATTLE_ROLE_NONE env, got %d", roleNoneCount)
	}
	if strictValue, ok := envVar(env, "STRICT_VERIFY"); !ok || strictValue != "custom-strict" {
		t.Fatalf("expected STRICT_VERIFY=custom-strict, got %q", strictValue)
	}
}

func TestUninstallPlanReadyRequiresRolloutLabelOnEveryTargetedNode(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-a"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-1",
				Labels: map[string]string{},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if ready {
		t.Fatalf("expected uninstall plan to wait while a targeted node is missing rollout label")
	}
	if !strings.Contains(msg, "node node-1 missing") {
		t.Fatalf("expected missing-node rollout label message, got %q", msg)
	}
}

func TestUninstallPlanReadyReturnsTrueWhenAllTargetedNodesHaveRolloutLabel(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-b"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if !ready {
		t.Fatalf("expected uninstall plan to be ready, got %q", msg)
	}
}

func TestUninstallPlanReadyRejectsStaleRolloutLabel(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-c"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: "stale-rollout-id",
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if ready {
		t.Fatalf("expected uninstall plan to reject stale node rollout labels")
	}
	if !strings.Contains(msg, "node node-1 missing") {
		t.Fatalf("expected stale-label wait message, got %q", msg)
	}
}

func TestUninstallPlanReadySkipsWindowsPlanWithNoTargets(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-d"
	plan := completeUninstallPlan(SystemAgentUpgraderWindowsPlanName, rolloutID)
	upgradev1.PlanComplete.False(plan)
	plan.Status.Applying = []string{"node-1"}

	ready, msg := uninstallPlanReady(plan, rolloutID, nil)
	if !ready {
		t.Fatalf("expected windows uninstall plan with zero targets to be skipped, got %q", msg)
	}
}

func TestUninstallPlanReadyDoesNotSkipLinuxPlanWithNoTargets(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-linux-no-targets"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)

	ready, msg := uninstallPlanReady(plan, rolloutID, nil)
	if ready {
		t.Fatalf("expected linux uninstall plan with zero targets to remain pending")
	}
	if !strings.Contains(msg, "no targeted nodes observed yet") {
		t.Fatalf("expected no-target wait message, got %q", msg)
	}
}

func TestUninstallPlanReadyRejectsWrongRunID(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-e"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, "different-rollout-id")
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if ready {
		t.Fatalf("expected uninstall plan to reject mismatched run ID")
	}
	if !strings.Contains(msg, systemAgentUpgraderRunIDEnvName) {
		t.Fatalf("expected run-id mismatch message, got %q", msg)
	}
}

func TestUninstallPlanReadyRejectsWrongPostCompleteLabel(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-f"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)
	plan.Spec.PostCompleteLabels[systemAgentUpgraderRolloutIDLabel] = "different-rollout-id"
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if ready {
		t.Fatalf("expected uninstall plan to reject mismatched postCompleteLabels rollout ID")
	}
	if !strings.Contains(msg, "postCompleteLabels") {
		t.Fatalf("expected postCompleteLabels mismatch message, got %q", msg)
	}
}

func TestUninstallPlanReadyBlocksWhileApplying(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-g"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)
	plan.Status.Applying = []string{"node-1"}
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if ready {
		t.Fatalf("expected uninstall plan to block while status.applying is not empty")
	}
	if !strings.Contains(msg, "still applying") {
		t.Fatalf("expected applying wait message, got %q", msg)
	}
}

func TestUninstallPlanReadyBlocksWhenPlanNotComplete(t *testing.T) {
	t.Parallel()

	const rolloutID = "rollout-id-h"
	plan := completeUninstallPlan(SystemAgentUpgraderPlanName, rolloutID)
	upgradev1.PlanComplete.False(plan)
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
				Labels: map[string]string{
					systemAgentUpgraderRolloutIDLabel: rolloutID,
				},
			},
		},
	}

	ready, msg := uninstallPlanReady(plan, rolloutID, nodes)
	if ready {
		t.Fatalf("expected uninstall plan to block while PlanComplete is false")
	}
	if !strings.Contains(msg, "completion condition not met") {
		t.Fatalf("expected completion wait message, got %q", msg)
	}
}

func TestEnsureUninstallRolloutReusesExistingID(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c-m-test",
			Annotations: map[string]string{
				importedUninstallRolloutIDAnnotation: "rollout-existing",
			},
		},
	}

	fakeClusters := &fakeClusterController{}
	h := &handler{
		clusters: fakeClusters,
	}

	updated, rolloutID, created, err := h.ensureUninstallRollout(cluster)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if created {
		t.Fatalf("expected existing rollout ID to be reused without creation")
	}
	if rolloutID != "rollout-existing" {
		t.Fatalf("expected rollout-existing, got %q", rolloutID)
	}
	if updated != cluster {
		t.Fatalf("expected same cluster object when rollout ID already exists")
	}
	if fakeClusters.updateCalls != 0 {
		t.Fatalf("expected no cluster update when rollout ID exists")
	}

	updated, rolloutID, created, err = h.ensureUninstallRollout(updated)
	if err != nil {
		t.Fatalf("expected no error on repeated call, got %v", err)
	}
	if created || rolloutID != "rollout-existing" {
		t.Fatalf("expected stable reused rollout ID, got created=%t rolloutID=%q", created, rolloutID)
	}
	if fakeClusters.updateCalls != 0 {
		t.Fatalf("expected no updates across repeated reuse calls")
	}
}

func TestEnsureUninstallRolloutCreatesMissingIDAndThenReuses(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c-m-test",
		},
	}
	fakeClusters := &fakeClusterController{}
	h := &handler{
		clusters: fakeClusters,
	}

	updated, rolloutID, created, err := h.ensureUninstallRollout(cluster)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !created {
		t.Fatalf("expected rollout ID to be created for missing annotation")
	}
	if rolloutID == "" {
		t.Fatalf("expected created rollout ID to be non-empty")
	}
	if updated.Annotations[importedUninstallRolloutIDAnnotation] != rolloutID {
		t.Fatalf("expected rollout ID annotation %q, got %q", rolloutID, updated.Annotations[importedUninstallRolloutIDAnnotation])
	}
	if fakeClusters.updateCalls != 1 {
		t.Fatalf("expected one update call for rollout creation, got %d", fakeClusters.updateCalls)
	}

	updatedAgain, rolloutIDAgain, createdAgain, err := h.ensureUninstallRollout(updated)
	if err != nil {
		t.Fatalf("expected no error on reuse call, got %v", err)
	}
	if createdAgain {
		t.Fatalf("expected created=false when rollout ID already exists")
	}
	if rolloutIDAgain != rolloutID {
		t.Fatalf("expected rollout ID reuse %q, got %q", rolloutID, rolloutIDAgain)
	}
	if updatedAgain != updated {
		t.Fatalf("expected same updated cluster object on reuse path")
	}
	if fakeClusters.updateCalls != 1 {
		t.Fatalf("expected no additional update call after reuse, got %d", fakeClusters.updateCalls)
	}
}

func TestReconcileImportedDisableUninstallCreatesRolloutAndRequeuesBeforePlanApply(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "c-m-test",
			Annotations: map[string]string{
				importedCleaningStateAnnotation: apimgmtv3.ImportedDay2OpsCleaningStateUninstall,
			},
		},
	}
	fakeClusters := &fakeClusterController{}
	h := &handler{
		clusters: fakeClusters,
	}

	updated, err := h.reconcileImportedDisable(cluster)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Annotations[importedUninstallRolloutIDAnnotation] == "" {
		t.Fatalf("expected rollout ID annotation to be persisted before continuing uninstall")
	}
	if fakeClusters.updateCalls != 1 {
		t.Fatalf("expected one update call to persist rollout ID, got %d", fakeClusters.updateCalls)
	}
	if fakeClusters.enqueueAfterCalls != 1 {
		t.Fatalf("expected one requeue after rollout creation boundary, got %d", fakeClusters.enqueueAfterCalls)
	}
	if fakeClusters.lastEnqueueName != "c-m-test" {
		t.Fatalf("expected enqueue for c-m-test, got %q", fakeClusters.lastEnqueueName)
	}
	if fakeClusters.lastEnqueueDuration != importedDay2OpsDisableRequeueInterval {
		t.Fatalf("expected requeue duration %s, got %s", importedDay2OpsDisableRequeueInterval, fakeClusters.lastEnqueueDuration)
	}
}

func TestPlanTargetedNodesRequiresHostnameLabel(t *testing.T) {
	t.Parallel()

	ctx := &config.UserContext{
		K8sClient: k8sfake.NewSimpleClientset(
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-with-hostname",
					Labels: map[string]string{
						corev1.LabelOSStable: "linux",
						corev1.LabelHostname: "node-with-hostname",
						"custom-role":        "worker",
						"another-test-label": "value",
					},
				},
			},
			&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-without-hostname",
					Labels: map[string]string{
						corev1.LabelOSStable: "linux",
					},
				},
			},
		),
	}
	h := &handler{}
	plan := &upgradev1.Plan{
		Spec: upgradev1.PlanSpec{
			NodeSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					corev1.LabelOSStable: "linux",
				},
			},
		},
	}

	nodes, err := h.planTargetedNodes(ctx, plan)
	if err != nil {
		t.Fatalf("expected no error listing targeted nodes, got %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "node-with-hostname" {
		t.Fatalf("expected only hostname-labeled node to be targeted, got %+v", nodes)
	}
}

func TestHasOperationsReturnsTrueForClusterNamespace(t *testing.T) {
	t.Parallel()

	saveCache := &fakeETCDSnapshotSaveCache{
		items: []*opv1alpha1.ETCDSnapshotSave{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-save"}},
		},
	}
	h := &handler{
		etcdSnapshotSaveCache: saveCache,
	}

	hasOps, err := h.hasOperations("c-m-test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !hasOps {
		t.Fatalf("expected hasOperations to return true")
	}
	if saveCache.lastNamespace != "c-m-test" {
		t.Fatalf("expected namespace-scoped list in c-m-test, got %q", saveCache.lastNamespace)
	}
}

func TestDeleteOperationsDeletesNamespacedOperationsAndSkipsDeleting(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	saveCache := &fakeETCDSnapshotSaveCache{
		items: []*opv1alpha1.ETCDSnapshotSave{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-save"}},
		},
	}
	restoreCache := &fakeETCDSnapshotRestoreCache{
		items: []*opv1alpha1.ETCDSnapshotRestore{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-restore", DeletionTimestamp: &now}},
		},
	}
	rotationCache := &fakeEncryptionKeyRotationCache{
		items: []*opv1alpha1.EncryptionKeyRotation{
			{ObjectMeta: metav1.ObjectMeta{Namespace: "c-m-test", Name: "op-rotation"}},
		},
	}

	saveClient := &fakeETCDSnapshotSaveClient{}
	restoreClient := &fakeETCDSnapshotRestoreClient{}
	rotationClient := &fakeEncryptionKeyRotationClient{}

	h := &handler{
		etcdSnapshotSaveCache:    saveCache,
		etcdSnapshotRestoreCache: restoreCache,
		encryptionRotationCache:  rotationCache,
		etcdSnapshotSaves:        saveClient,
		etcdSnapshotRestores:     restoreClient,
		encryptionRotations:      rotationClient,
	}

	remaining, err := h.deleteOperations("c-m-test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !remaining {
		t.Fatalf("expected remaining=true while operations still exist or are deleting")
	}
	if len(saveClient.deleted) != 1 || saveClient.deleted[0] != (namespacedName{namespace: "c-m-test", name: "op-save"}) {
		t.Fatalf("expected save delete for c-m-test/op-save, got %+v", saveClient.deleted)
	}
	if len(rotationClient.deleted) != 1 || rotationClient.deleted[0] != (namespacedName{namespace: "c-m-test", name: "op-rotation"}) {
		t.Fatalf("expected rotation delete for c-m-test/op-rotation, got %+v", rotationClient.deleted)
	}
	if len(restoreClient.deleted) != 0 {
		t.Fatalf("expected deleting restore to be skipped, got deletes %+v", restoreClient.deleted)
	}
	if saveCache.lastNamespace != "c-m-test" || restoreCache.lastNamespace != "c-m-test" || rotationCache.lastNamespace != "c-m-test" {
		t.Fatalf("expected namespace-scoped list for all operation kinds")
	}
}

func TestDeleteOperationsReturnsFalseWhenNoOperationsExist(t *testing.T) {
	t.Parallel()

	h := &handler{
		etcdSnapshotSaveCache:    &fakeETCDSnapshotSaveCache{},
		etcdSnapshotRestoreCache: &fakeETCDSnapshotRestoreCache{},
		encryptionRotationCache:  &fakeEncryptionKeyRotationCache{},
		etcdSnapshotSaves:        &fakeETCDSnapshotSaveClient{},
		etcdSnapshotRestores:     &fakeETCDSnapshotRestoreClient{},
		encryptionRotations:      &fakeEncryptionKeyRotationClient{},
	}

	remaining, err := h.deleteOperations("c-m-empty")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if remaining {
		t.Fatalf("expected remaining=false when no operations exist")
	}
}

func TestDisableNeededReturnsTrueForLeftoverImportedPlanIdentity(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "c-m-test"}}

	h := &handler{
		beaconCache:              &fakeBeaconCache{notFound: true},
		etcdSnapshotSaveCache:    &fakeETCDSnapshotSaveCache{},
		etcdSnapshotRestoreCache: &fakeETCDSnapshotRestoreCache{},
		encryptionRotationCache:  &fakeEncryptionKeyRotationCache{},
		secretCache: &fakeSecretCache{
			items: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "c-m-test",
						Name:      "leftover-machine-plan-token",
						Labels: map[string]string{
							serviceaccounttoken.ServiceAccountSecretLabel: "leftover-machine-plan",
						},
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
		},
		serviceAccountCache: &fakeServiceAccountCache{},
		roles: &fakeRoleClient{
			items: []rbacv1.Role{
				{ObjectMeta: metav1.ObjectMeta{Name: "leftover-machine-plan", Namespace: "c-m-test"}},
			},
		},
		roleBindings: &fakeRoleBindingClient{
			items: []rbacv1.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "leftover-machine-plan", Namespace: "c-m-test"},
					Subjects: []rbacv1.Subject{
						{Kind: "ServiceAccount", Name: "leftover-machine-plan", Namespace: "c-m-test"},
					},
				},
			},
		},
	}

	needed, err := h.disableNeeded(cluster)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !needed {
		t.Fatalf("expected disableNeeded to keep reconciling for leftover imported plan identity resources")
	}
}

func TestDeleteImportedPlanIdentityDeletesOnlyServiceAccountTokenSecrets(t *testing.T) {
	t.Parallel()

	h := &handler{
		secretCache: &fakeSecretCache{
			items: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "c-m-test",
						Name:      "plan-token",
						Labels: map[string]string{
							serviceaccounttoken.ServiceAccountSecretLabel: "node-machine-plan",
						},
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "c-m-test",
						Name:      "plan-opaque",
						Labels: map[string]string{
							serviceaccounttoken.ServiceAccountSecretLabel: "node-machine-plan",
						},
					},
					Type: corev1.SecretTypeOpaque,
				},
			},
		},
		secrets:         &fakeSecretClient{},
		roleBindings:    &fakeRoleBindingClient{},
		roles:           &fakeRoleClient{},
		serviceAccounts: &fakeServiceAccountClient{},
	}

	err := h.deleteImportedPlanIdentity(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-machine-plan",
			Namespace: "c-m-test",
			Labels: map[string]string{
				capr.RoleLabel:        capr.RolePlan,
				capr.ClusterNameLabel: "c-m-test",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	secretClient := h.secrets.(*fakeSecretClient)
	if len(secretClient.deleted) != 1 || secretClient.deleted[0] != (namespacedName{namespace: "c-m-test", name: "plan-token"}) {
		t.Fatalf("expected only service-account token secret to be deleted, got %+v", secretClient.deleted)
	}
}

func completeUninstallPlan(name, rolloutID string) *upgradev1.Plan {
	plan := &upgradev1.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespaces.System,
		},
		Spec: upgradev1.PlanSpec{
			Upgrade: &upgradev1.ContainerSpec{
				Env: []corev1.EnvVar{
					{Name: "UNINSTALL", Value: "true"},
					{Name: systemAgentUpgraderRunIDEnvName, Value: rolloutID},
				},
			},
			PostCompleteLabels: map[string]string{
				systemAgentUpgraderRolloutIDLabel: rolloutID,
			},
		},
	}
	upgradev1.PlanComplete.True(plan)
	return plan
}

func renderedPlans(objs []runtime.Object) []*upgradev1.Plan {
	plans := make([]*upgradev1.Plan, 0, len(objs))
	for i := range objs {
		plan, ok := objs[i].(*upgradev1.Plan)
		if ok {
			plans = append(plans, plan)
		}
	}
	return plans
}

func envVar(env []corev1.EnvVar, name string) (string, bool) {
	for i := range env {
		if env[i].Name == name {
			return env[i].Value, true
		}
	}
	return "", false
}

func envVarCount(env []corev1.EnvVar, name string) int {
	count := 0
	for i := range env {
		if env[i].Name == name {
			count++
		}
	}
	return count
}

func hasPlanName(plans []*upgradev1.Plan, name string) bool {
	for i := range plans {
		if plans[i].Name == name {
			return true
		}
	}
	return false
}

type namespacedName struct {
	namespace string
	name      string
}

type fakeClusterController struct {
	mgmtcontrollers.ClusterController
	updateCalls         int
	enqueueAfterCalls   int
	lastEnqueueName     string
	lastEnqueueDuration time.Duration
}

func (f *fakeClusterController) Update(cluster *apimgmtv3.Cluster) (*apimgmtv3.Cluster, error) {
	f.updateCalls++
	return cluster, nil
}

func (f *fakeClusterController) EnqueueAfter(name string, duration time.Duration) {
	f.enqueueAfterCalls++
	f.lastEnqueueName = name
	f.lastEnqueueDuration = duration
}

type fakeETCDSnapshotSaveCache struct {
	operationcontrollers.ETCDSnapshotSaveCache
	items           []*opv1alpha1.ETCDSnapshotSave
	err             error
	lastNamespace   string
	lastHasSelector bool
}

func (f *fakeETCDSnapshotSaveCache) List(namespace string, selector labels.Selector) ([]*opv1alpha1.ETCDSnapshotSave, error) {
	f.lastNamespace = namespace
	f.lastHasSelector = selector != nil
	return f.items, f.err
}

type fakeETCDSnapshotRestoreCache struct {
	operationcontrollers.ETCDSnapshotRestoreCache
	items           []*opv1alpha1.ETCDSnapshotRestore
	err             error
	lastNamespace   string
	lastHasSelector bool
}

func (f *fakeETCDSnapshotRestoreCache) List(namespace string, selector labels.Selector) ([]*opv1alpha1.ETCDSnapshotRestore, error) {
	f.lastNamespace = namespace
	f.lastHasSelector = selector != nil
	return f.items, f.err
}

type fakeEncryptionKeyRotationCache struct {
	operationcontrollers.EncryptionKeyRotationCache
	items           []*opv1alpha1.EncryptionKeyRotation
	err             error
	lastNamespace   string
	lastHasSelector bool
}

func (f *fakeEncryptionKeyRotationCache) List(namespace string, selector labels.Selector) ([]*opv1alpha1.EncryptionKeyRotation, error) {
	f.lastNamespace = namespace
	f.lastHasSelector = selector != nil
	return f.items, f.err
}

type fakeETCDSnapshotSaveClient struct {
	operationcontrollers.ETCDSnapshotSaveClient
	deleted []namespacedName
	err     error
}

func (f *fakeETCDSnapshotSaveClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeETCDSnapshotRestoreClient struct {
	operationcontrollers.ETCDSnapshotRestoreClient
	deleted []namespacedName
	err     error
}

func (f *fakeETCDSnapshotRestoreClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeEncryptionKeyRotationClient struct {
	operationcontrollers.EncryptionKeyRotationClient
	deleted []namespacedName
	err     error
}

func (f *fakeEncryptionKeyRotationClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeBeaconCache struct {
	plancontrollers.BeaconCache
	beacon    *planv1alpha1.Beacon
	err       error
	notFound  bool
	namespace string
	name      string
}

func (f *fakeBeaconCache) Get(namespace, name string) (*planv1alpha1.Beacon, error) {
	f.namespace = namespace
	f.name = name
	if f.notFound {
		return nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    planv1alpha1.SchemeGroupVersion.Group,
			Resource: "beacons",
		}, name)
	}
	return f.beacon, f.err
}

type fakeSecretCache struct {
	corecontrollers.SecretCache
	items []*corev1.Secret
	err   error
}

func (f *fakeSecretCache) List(string, labels.Selector) ([]*corev1.Secret, error) {
	return f.items, f.err
}

type fakeSecretClient struct {
	corecontrollers.SecretClient
	deleted []namespacedName
	err     error
}

func (f *fakeSecretClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeServiceAccountCache struct {
	corecontrollers.ServiceAccountCache
	items []*corev1.ServiceAccount
	err   error
}

func (f *fakeServiceAccountCache) List(string, labels.Selector) ([]*corev1.ServiceAccount, error) {
	return f.items, f.err
}

type fakeServiceAccountClient struct {
	corecontrollers.ServiceAccountClient
	deleted []namespacedName
	err     error
}

func (f *fakeServiceAccountClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeRoleClient struct {
	rbaccontrollers.RoleClient
	items   []rbacv1.Role
	deleted []namespacedName
	err     error
}

func (f *fakeRoleClient) List(string, metav1.ListOptions) (*rbacv1.RoleList, error) {
	return &rbacv1.RoleList{Items: f.items}, f.err
}

func (f *fakeRoleClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}

type fakeRoleBindingClient struct {
	rbaccontrollers.RoleBindingClient
	items   []rbacv1.RoleBinding
	deleted []namespacedName
	err     error
}

func (f *fakeRoleBindingClient) List(string, metav1.ListOptions) (*rbacv1.RoleBindingList, error) {
	return &rbacv1.RoleBindingList{Items: f.items}, f.err
}

func (f *fakeRoleBindingClient) Delete(namespace, name string, _ *metav1.DeleteOptions) error {
	f.deleted = append(f.deleted, namespacedName{namespace: namespace, name: name})
	return f.err
}
