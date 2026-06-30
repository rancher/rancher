package etcdsnapshotrestore

import (
	"encoding/base64"
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

func (a *stubAdapter) WaitForRegister() (bool, error)                    { return a.waitForRegisterOK, nil }
func (a *stubAdapter) PauseCluster(_ bool) error                         { return nil }
func (a *stubAdapter) RuntimeCommand() string                            { return a.runtimeCommand }
func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) string       { return a.dataDir }
func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string { return a.provisioningDir }
func (a *stubAdapter) ServerUnit() string                                { return a.serverUnit }
func (a *stubAdapter) RenderProbes(_ *corev1.Secret, _ bool) (map[string]rkeplan.Probe, error) {
	return map[string]rkeplan.Probe{}, nil
}
func (a *stubAdapter) KubectlPath(_ *corev1.Secret) string    { return a.kubectlPath }
func (a *stubAdapter) KubeconfigPath(_ *corev1.Secret) string { return a.kubeconfigPath }
func (a *stubAdapter) FindOrElectLeader(_ string, _ ops.Filter) (*corev1.Secret, error) {
	return nil, nil
}

// The five methods below complete the ops.Adapter contract for the stub. They are not exercised
// by the snapshot-restore controller (which only consumes runtime/dataDir/serverUnit/probes/
// kubectl+kubeconfig paths/plans), so each returns a static, runtime-appropriate value.
func (a *stubAdapter) ConfigDirectory(_ *corev1.Secret) string {
	return "/etc/rancher/" + a.runtimeCommand + "/config.yaml.d"
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
