package etcdsnapshotrestore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// stubAdapter is a minimal ops.Adapter implementation for testing plan construction.
// Methods unrelated to the test return zero values.
type stubAdapter struct {
	runtimeCommand    string
	dataDir           string
	provisioningDir   string
	kubectlPath       string
	kubeconfigPath    string
	serverUnit        string
	waitForRegisterOK bool
}

func (a *stubAdapter) EtcdSnapshotNamespace() string {
	return "test-namespace"
}

func (a *stubAdapter) ClusterObject() (*unstructured.Unstructured, error) {
	//TODO implement me
	panic("implement me")
}

func (a *stubAdapter) BeaconRef() (string, string)                       { return "test-namespace", "test-cluster" }
func (a *stubAdapter) WaitForRegister() (bool, error)                    { return a.waitForRegisterOK, nil }
func (a *stubAdapter) PauseCluster(_ bool) error                         { return nil }
func (a *stubAdapter) RuntimeCommand() string                            { return a.runtimeCommand }
func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) string       { return a.dataDir }
func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string { return a.provisioningDir }
func (a *stubAdapter) ServerUnit() string                                { return a.serverUnit }
func (a *stubAdapter) RuntimeService(_ *corev1.Secret) string            { return a.serverUnit }
func (a *stubAdapter) DistroServices(secret *corev1.Secret) []string {
	return ops.DistroServices(a.runtimeCommand, secret)
}
func (a *stubAdapter) RenderProbes(_ *corev1.Secret, _ bool) (map[string]rkeplan.Probe, error) {
	return map[string]rkeplan.Probe{}, nil
}
func (a *stubAdapter) KubectlPath(_ *corev1.Secret) string    { return a.kubectlPath }
func (a *stubAdapter) KubeconfigPath(_ *corev1.Secret) string { return a.kubeconfigPath }
func (a *stubAdapter) FindOrElectLeader(_ string, _ ops.Filter) (*corev1.Secret, error) {
	return nil, nil
}

// The six methods below complete the ops.Adapter contract for the stub. They are not exercised
// by the snapshot-restore controller (which only consumes runtime/dataDir/serverUnit/probes/
// kubectl+kubeconfig paths/plans), so each returns a static, runtime-appropriate value.
func (a *stubAdapter) ConfigFile(_ *corev1.Secret) string {
	return "/etc/rancher/" + a.runtimeCommand + "/config.yaml"
}
func (a *stubAdapter) ConfigDirectory(_ *corev1.Secret) string {
	return "/etc/rancher/" + a.runtimeCommand + "/config.yaml.d"
}
func (a *stubAdapter) ComponentTLSSettings(_ *corev1.Secret, _ string) (ops.ComponentTLSSettings, error) {
	return ops.ComponentTLSSettings{}, nil
}
func (a *stubAdapter) GetServerURL(_ *corev1.Secret) string      { return "" }
func (a *stubAdapter) GetSupervisorPort(_ *corev1.Secret) string { return "9345" }
func (a *stubAdapter) LoopbackAddress(_ *corev1.Secret) string   { return "127.0.0.1" }
func (a *stubAdapter) ToS3ArgsEnvAndFiles(_ *corev1.Secret) ([]string, []string, []planapi.File) {
	return nil, nil, nil
}

func newTestScope(adapter *stubAdapter, uid types.UID) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetName("test-cluster")
	cluster.SetNamespace("fleet-default")
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")

	return &scope{
		op: &opv1alpha1.ETCDSnapshotRestore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "restore-1",
				Namespace: "fleet-default",
				UID:       uid,
			},
		},
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    adapter,
	}
}

func defaultAdapter() *stubAdapter {
	return &stubAdapter{
		runtimeCommand:  "rke2",
		dataDir:         "/var/lib/rancher/rke2",
		provisioningDir: "/var/lib/rancher/capr",
		kubectlPath:     "/var/lib/rancher/rke2/bin/kubectl",
		kubeconfigPath:  "/etc/rancher/rke2/rke2.yaml",
		serverUnit:      "rke2-server",
	}
}

func makePlanSecret(name, nodeName string, labels map[string]string) *corev1.Secret {
	if labels == nil {
		labels = map[string]string{}
	}
	labels[capr.ClusterNameLabel] = "test-cluster"
	if nodeName != "" {
		labels[capr.NodeNameLabel] = nodeName
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "fleet-default",
			Labels:    labels,
			UID:       types.UID(name + "-uid"),
		},
	}
}

func TestBuildPostRestoreNodeCleanupPlan(t *testing.T) {
	t.Parallel()

	s := newTestScope(defaultAdapter(), "restore-uid")
	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel: "true",
		capr.InitNodeLabel: "true",
	})
	other := makePlanSecret("worker-1", "node-worker-1", map[string]string{
		capr.WorkerRoleLabel: "true",
	})
	allSecrets := []*corev1.Secret{initSecret, other}

	plan, skipReason := buildPostRestoreNodeCleanupPlan(s, initSecret, allSecrets)
	if skipReason != "" {
		t.Fatalf("unexpected skipReason: %q", skipReason)
	}
	if plan == nil {
		t.Fatal("expected non-nil plan")
	}

	// 3 files: idempotent script, cleanup script, node names list.
	if len(plan.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(plan.Files))
	}

	wantIdempotentPath := ops.IdempotentActionScriptPath(s.adapter.ProvisioningDataDirectory(initSecret))
	wantCleanupPath := path.Join(s.adapter.ProvisioningDataDirectory(initSecret), etcdRestoreBinSubdir, nodeCleanupScriptName)
	wantNodeNamesPath := path.Join(s.adapter.ProvisioningDataDirectory(initSecret), etcdRestoreBinSubdir, fmt.Sprintf("node-names-%s", string(s.op.UID)))

	pathsByPath := map[string]planapi.File{}
	for _, f := range plan.Files {
		pathsByPath[f.Path] = f
	}
	for _, p := range []string{wantIdempotentPath, wantCleanupPath, wantNodeNamesPath} {
		if _, ok := pathsByPath[p]; !ok {
			t.Errorf("missing file at path %q", p)
		}
	}

	nodeNamesFile := pathsByPath[wantNodeNamesPath]
	decoded, err := base64.StdEncoding.DecodeString(nodeNamesFile.Content)
	if err != nil {
		t.Fatalf("node names file content not valid base64: %v", err)
	}
	wantNodeNames := "node-init\nnode-worker-1\n"
	if string(decoded) != wantNodeNames {
		t.Errorf("node names content = %q, want %q", string(decoded), wantNodeNames)
	}

	if !nodeNamesFile.Dynamic {
		t.Error("node names file should be Dynamic (one cleanup per restore)")
	}

	cleanupScriptFile := pathsByPath[wantCleanupPath]
	decodedScript, err := base64.StdEncoding.DecodeString(cleanupScriptFile.Content)
	if err != nil {
		t.Fatalf("cleanup script content not valid base64: %v", err)
	}
	if string(decodedScript) != nodeCleanupScript {
		t.Errorf("cleanup script content does not match nodeCleanupScript")
	}

	if len(plan.OneTimeInstructions) != 1 {
		t.Fatalf("expected 1 instruction, got %d", len(plan.OneTimeInstructions))
	}
	instr := plan.OneTimeInstructions[0]
	if instr.Command != "/bin/sh" {
		t.Errorf("instruction Command = %q, want /bin/sh", instr.Command)
	}
	// The script invocation must reference the cleanup script path and the node names file path.
	joined := strings.Join(instr.Args, " ")
	if !strings.Contains(joined, wantCleanupPath) {
		t.Errorf("instruction args do not reference cleanup script path %q: %v", wantCleanupPath, instr.Args)
	}
	if !strings.Contains(joined, wantNodeNamesPath) {
		t.Errorf("instruction args do not reference node names path %q: %v", wantNodeNamesPath, instr.Args)
	}

	// The KUBECTL/KUBECONFIG env entries must be set so the cleanup script can find its tools.
	envSet := map[string]bool{}
	for _, e := range instr.Env {
		envSet[e] = true
	}
	if !envSet["KUBECTL="+s.adapter.KubectlPath(initSecret)] {
		t.Errorf("KUBECTL env missing or wrong: %v", instr.Env)
	}
	if !envSet["KUBECONFIG="+s.adapter.KubeconfigPath(initSecret)] {
		t.Errorf("KUBECONFIG env missing or wrong: %v", instr.Env)
	}

	// The instruction must be wrapped in the idempotent script — the script path appears as the
	// second arg (after -x).
	if len(instr.Args) < 2 || instr.Args[1] != wantIdempotentPath {
		t.Errorf("instruction is not idempotent-wrapped, Args[1] = %v", instr.Args)
	}
}

func TestBuildPostRestoreNodeCleanupPlanSkipsWhenNoNodeNames(t *testing.T) {
	t.Parallel()

	s := newTestScope(defaultAdapter(), "restore-uid")
	initSecret := makePlanSecret("init", "", map[string]string{
		capr.EtcdRoleLabel: "true",
		capr.InitNodeLabel: "true",
	})
	// initSecret has no node-name label; allSecrets list has only this secret.
	plan, skipReason := buildPostRestoreNodeCleanupPlan(s, initSecret, []*corev1.Secret{initSecret})
	if plan != nil {
		t.Error("expected nil plan when there are no node names to preserve")
	}
	if skipReason == "" {
		t.Error("expected non-empty skipReason when there are no node names")
	}
}

func TestBuildPostRestoreNodeCleanupPlanSkipsWhenNoKubectl(t *testing.T) {
	t.Parallel()

	a := defaultAdapter()
	a.kubectlPath = ""
	s := newTestScope(a, "restore-uid")
	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel: "true",
		capr.InitNodeLabel: "true",
	})
	plan, skipReason := buildPostRestoreNodeCleanupPlan(s, initSecret, []*corev1.Secret{initSecret})
	if plan != nil {
		t.Error("expected nil plan when kubectl path is missing")
	}
	if skipReason == "" {
		t.Error("expected non-empty skipReason when kubectl path is missing")
	}
}

func TestIdempotencyValueStable(t *testing.T) {
	t.Parallel()

	s := newTestScope(defaultAdapter(), "abc-123")
	if got := s.idempotencyValue(); got != "abc-123" {
		t.Errorf("idempotencyValue = %q, want %q", got, "abc-123")
	}
}

func TestBuildPreflightPlan(t *testing.T) {
	t.Parallel()

	s := newTestScope(defaultAdapter(), "restore-uid")
	secret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel: "true",
		capr.InitNodeLabel: "true",
	})

	plan := buildPreflightPlan(s, secret)

	if len(plan.OneTimeInstructions) != 1 {
		t.Fatalf("expected 1 instruction, got %d", len(plan.OneTimeInstructions))
	}
	instr := plan.OneTimeInstructions[0]
	// The name is the key reconcilePreflight reads the token hash back under.
	if instr.Name != preflightInstructionName {
		t.Errorf("instruction Name = %q, want %q", instr.Name, preflightInstructionName)
	}
	if !instr.SaveOutput {
		t.Error("SaveOutput must be set; the step compares the instruction's output against the snapshot")
	}

	// Wrapping the check in the idempotent script would print a message in place of the hash on an
	// attempt it considers already reconciled.
	if len(instr.Args) > 1 && instr.Args[1] == ops.IdempotentActionScriptPath(s.adapter.ProvisioningDataDirectory(secret)) {
		t.Error("the preflight check must not be idempotent-wrapped; it has to run on every attempt")
	}
}

func TestBuildShutdownPlan(t *testing.T) {
	t.Parallel()

	s := newTestScope(defaultAdapter(), "restore-uid")
	adapter := defaultAdapter()

	t.Run("etcd and control plane node", func(t *testing.T) {
		secret := makePlanSecret("init", "node-init", map[string]string{
			capr.EtcdRoleLabel:         "true",
			capr.ControlPlaneRoleLabel: "true",
		})

		plan := buildShutdownPlan(s, secret)

		var names []string
		for _, instr := range plan.OneTimeInstructions {
			names = append(names, instr.Name)
		}
		want := []string{"remove idempotency tracking", "shutdown", "create-etcd-tombstone", "remove-tls-directory"}
		if strings.Join(names, ",") != strings.Join(want, ",") {
			t.Errorf("instructions = %v, want %v", names, want)
		}

		// The killall script reads the data directory out of the environment.
		wantEnv := fmt.Sprintf("%s_DATA_DIR=%s", strings.ToUpper(adapter.RuntimeCommand()), adapter.DistroDataDirectory(secret))
		var found bool
		for _, e := range plan.OneTimeInstructions[1].Env {
			if e == wantEnv {
				found = true
			}
		}
		if !found {
			t.Errorf("shutdown instruction env = %v, want it to contain %q", plan.OneTimeInstructions[1].Env, wantEnv)
		}

		if len(plan.Files) != 1 || plan.Files[0].Path != ops.IdempotentActionScriptPath(adapter.ProvisioningDataDirectory(secret)) {
			t.Errorf("expected the idempotent script file, got %v", plan.Files)
		}
	})

	t.Run("worker node", func(t *testing.T) {
		secret := makePlanSecret("worker-1", "node-worker-1", map[string]string{
			capr.WorkerRoleLabel: "true",
		})

		plan := buildShutdownPlan(s, secret)

		// No etcd data or TLS material to clear on a worker.
		if len(plan.OneTimeInstructions) != 2 {
			t.Fatalf("expected 2 instructions, got %d", len(plan.OneTimeInstructions))
		}
		for _, instr := range plan.OneTimeInstructions {
			if instr.Name == "create-etcd-tombstone" || instr.Name == "remove-tls-directory" {
				t.Errorf("unexpected instruction %q for a worker node", instr.Name)
			}
		}
	})
}

// TestAssignedPlansAreOperationScoped covers the property every plan this controller assigns depends
// on: the system-agent only re-runs a plan whose serialized content changed, and AssignPlan only
// writes a plan whose bytes differ from the one already on the secret. Two operations doing the same
// work must therefore serialize differently, or the second is reported as already applied — its
// instructions never run and its output is the first operation's. Reconciles of one operation must
// serialize identically, or the plan would churn and re-trigger its instructions.
func TestAssignedPlansAreOperationScoped(t *testing.T) {
	t.Parallel()

	secret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel:         "true",
		capr.ControlPlaneRoleLabel: "true",
	})

	builders := map[string]struct {
		build func(*scope) *planapi.Plan
		step  opv1alpha1.ETCDSnapshotRestoreStep
	}{
		"preflight": {
			build: func(s *scope) *planapi.Plan { return buildPreflightPlan(s, secret) },
			step:  opv1alpha1.ETCDSnapshotRestoreStepPreflight,
		},
		"shutdown": {
			build: func(s *scope) *planapi.Plan { return buildShutdownPlan(s, secret) },
			step:  opv1alpha1.ETCDSnapshotRestoreStepShutdown,
		},
	}

	for name, b := range builders {
		t.Run(name, func(t *testing.T) {
			marshal := func(uid types.UID) string {
				s := newTestScope(defaultAdapter(), uid)
				p := ops.WithOperationEnv(b.build(s), ops.OperationEnv(ControllerOwnerKey, s.op, b.step))
				data, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}

			if marshal("restore-uid-1") == marshal("restore-uid-2") {
				t.Error("plans for two operations serialize identically, so the second would be reported as already applied")
			}
			if marshal("restore-uid-1") != marshal("restore-uid-1") {
				t.Error("plans for one operation must serialize identically across reconciles")
			}
		})
	}
}
