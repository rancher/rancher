package systemagent

import (
	"testing"

	apimgmtv3 "github.com/rancher/rancher/pkg/apis/management.cattle.io/v3"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	upgradev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestResetBeaconWait(t *testing.T) {
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
			name: "owned by reset",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{planv1alpha1.BeaconOwnerLabel: resetBeaconOwnerKey},
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
			name: "active without owner does not block reset",
			beacon: &planv1alpha1.Beacon{
				Status: planv1alpha1.BeaconStatus{Active: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wait, message := resetBeaconWait(tt.beacon)
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
		importedCleaningStateAnnotation: importedCleaningStateOperations,
	}) {
		t.Fatalf("expected cleaning state to keep disable reconciliation sticky")
	}
}

func TestInstallerDoesNotForceDelete(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "DELETE", Value: "false"},
				},
			},
		},
	}

	plans := renderedPlans(installer(cluster, "stv-aggregation"))
	if len(plans) == 0 {
		t.Fatalf("expected rendered plans")
	}
	for _, plan := range plans {
		if value, ok := envVar(plan.Spec.Upgrade.Env, "DELETE"); ok && value == "true" {
			t.Fatalf("did not expect installer to force DELETE=true for plan %s", plan.Name)
		}
	}
}

func TestUninstallerSetsDeleteEnv(t *testing.T) {
	t.Parallel()

	cluster := &apimgmtv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "c-m-abc"},
		Spec: apimgmtv3.ClusterSpec{
			ClusterSpecBase: apimgmtv3.ClusterSpecBase{
				AgentEnvVars: []corev1.EnvVar{
					{Name: "FOO", Value: "bar"},
					{Name: "DELETE", Value: "false"},
				},
			},
		},
	}

	plans := renderedPlans(uninstaller(cluster, "stv-aggregation"))
	if len(plans) == 0 {
		t.Fatalf("expected rendered plans")
	}
	for _, plan := range plans {
		value, ok := envVar(plan.Spec.Upgrade.Env, "DELETE")
		if !ok || value != "true" {
			t.Fatalf("expected DELETE=true in uninstall plan %s", plan.Name)
		}
		if len(plan.Spec.Upgrade.Env) == 0 || plan.Spec.Upgrade.Env[0].Name != "DELETE" || plan.Spec.Upgrade.Env[0].Value != "true" {
			t.Fatalf("expected DELETE=true to be the first env in uninstall plan %s", plan.Name)
		}
		deleteCount := 0
		for _, env := range plan.Spec.Upgrade.Env {
			if env.Name == "DELETE" {
				deleteCount++
			}
		}
		if deleteCount != 1 {
			t.Fatalf("expected exactly one DELETE env in uninstall plan %s, got %d", plan.Name, deleteCount)
		}
	}
}

func TestPlanNames(t *testing.T) {
	t.Parallel()

	objs := []runtime.Object{
		upgradev1.NewPlan("cattle-system", SystemAgentUpgraderPlanName, upgradev1.Plan{}),
		&corev1.ServiceAccount{},
		upgradev1.NewPlan("cattle-system", SystemAgentUpgraderWindowsPlanName, upgradev1.Plan{}),
	}

	names := planNames(objs)
	if len(names) != 2 {
		t.Fatalf("expected 2 plan names, got %d", len(names))
	}
	if names[0] != SystemAgentUpgraderPlanName {
		t.Fatalf("expected first plan name %q, got %q", SystemAgentUpgraderPlanName, names[0])
	}
	if names[1] != SystemAgentUpgraderWindowsPlanName {
		t.Fatalf("expected second plan name %q, got %q", SystemAgentUpgraderWindowsPlanName, names[1])
	}
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
