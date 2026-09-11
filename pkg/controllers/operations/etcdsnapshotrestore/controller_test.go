package etcdsnapshotrestore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"slices"
	"strings"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	"github.com/rancher/rancher/pkg/restoremode"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// stubSnapshotClient serves a single ETCDSnapshot to the restore-mode code paths. Only Get is
// exercised; every other method of the generated client panics so an unexpected call is loud.
type stubSnapshotClient struct {
	rkecontrollers.ETCDSnapshotController

	snapshot *rkev1.ETCDSnapshot
	notFound bool
}

func (c *stubSnapshotClient) Get(_, name string, _ metav1.GetOptions) (*rkev1.ETCDSnapshot, error) {
	if c.notFound {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "rke.cattle.io", Resource: "etcdsnapshots"}, name)
	}
	return c.snapshot, nil
}

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

	// restoreTargets are the objects RestoreTarget serves, keyed by resource key. A key that is
	// absent yields (nil, nil), i.e. this cluster type has no counterpart for it.
	restoreTargets    map[string]*unstructured.Unstructured
	restoreTargetErr  error
	updatedTargets    []*unstructured.Unstructured
	updateRestoreErr  error
	restoreTargetKeys []string

	// restoreTargetSettled is what WaitForRestoreTarget reports. The zero value is false so a test
	// has to opt in to the settled state, mirroring a cluster that has not yet observed a write.
	restoreTargetSettled bool
	waitRestoreTargetErr error

	// installVersion, when set, is the Kubernetes version InstallInstruction installs. Empty means
	// this cluster type does not manage its distro version.
	installVersion string
}

func (a *stubAdapter) EtcdSnapshotNamespace() string {
	return "test-namespace"
}

func (a *stubAdapter) ClusterObject() (*unstructured.Unstructured, error) {
	//TODO implement me
	panic("implement me")
}

func (a *stubAdapter) RestoreTarget(resourceKey string) (*unstructured.Unstructured, error) {
	a.restoreTargetKeys = append(a.restoreTargetKeys, resourceKey)
	if a.restoreTargetErr != nil {
		return nil, a.restoreTargetErr
	}
	target, ok := a.restoreTargets[resourceKey]
	if !ok {
		return nil, nil
	}
	return target.DeepCopy(), nil
}

func (a *stubAdapter) UpdateRestoreTarget(obj *unstructured.Unstructured) error {
	if a.updateRestoreErr != nil {
		return a.updateRestoreErr
	}
	a.updatedTargets = append(a.updatedTargets, obj)
	// Reflect the write back so a subsequent RestoreTarget serves it, the way a real cache would
	// once the update lands. That is what lets a test drive the step to completion.
	for key, target := range a.restoreTargets {
		if target.GetKind() == obj.GetKind() && target.GetName() == obj.GetName() {
			a.restoreTargets[key] = obj
		}
	}
	return nil
}

func (a *stubAdapter) WaitForRestoreTarget() (bool, error) {
	return a.restoreTargetSettled, a.waitRestoreTargetErr
}

func (a *stubAdapter) InstallInstruction(secret *corev1.Secret) (planapi.OneTimeInstruction, bool) {
	if a.installVersion == "" {
		return planapi.OneTimeInstruction{}, false
	}
	return planapi.OneTimeInstruction{
		CommonInstruction: planapi.CommonInstruction{
			Name:    "install",
			Image:   "rancher/system-agent-installer-rke2:" + strings.ReplaceAll(a.installVersion, "+", "-"),
			Command: "sh",
			Args:    []string{"-c", "run.sh"},
			Env: []string{
				"INSTALL_RKE2_SKIP_START=true",
				"RKE2_DATA_DIR=" + a.DistroDataDirectory(secret),
			},
		},
	}, true
}

func (a *stubAdapter) BeaconRef() (string, string)                       { return "test-namespace", "test-cluster" }
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

// The six methods below complete the ops.Adapter contract for the stub. They are not exercised
// by the snapshot-restore controller (which only consumes runtime/dataDir/serverUnit/probes/
// kubectl+kubeconfig paths/plans), so each returns a static, runtime-appropriate value.
func (a *stubAdapter) ConfigFile(_ *corev1.Secret) string {
	return "/etc/rancher/" + a.runtimeCommand + "/config.yaml"
}
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

// provClusterTarget returns an unstructured provisioning cluster for the stub adapter to serve as
// the restore target, holding the pre-restore configuration.
func provClusterTarget() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "provisioning.cattle.io/v1",
		"kind":       "Cluster",
		"metadata": map[string]any{
			"name":      "test-cluster",
			"namespace": "fleet-default",
		},
		"spec": map[string]any{
			"kubernetesVersion": "v1.34.1+rke2r1",
			"rkeConfig": map[string]any{
				"additionalManifest": "# current",
			},
		},
	}}
}

// snapshotWithModes builds an upstream snapshot carrying an extra-metadata payload: resources plus a
// restoreModes map, encoded the way snapshotextrametadata and snapshotbackpopulate do.
func snapshotWithModes(t *testing.T, modes map[string]string, resources map[string]any, availableModes string) *rkev1.ETCDSnapshot {
	t.Helper()

	metadata := map[string]string{}

	if modes != nil {
		payload, err := json.Marshal(modes)
		if err != nil {
			t.Fatalf("marshalling restoreModes: %v", err)
		}
		metadata[rkev1.SnapshotMetadataRestoreModesKey] = string(payload)
	}

	if resources != nil {
		payload, err := snapshotutil.CompressInterface(resources)
		if err != nil {
			t.Fatalf("compressing resources: %v", err)
		}
		metadata[rkev1.SnapshotMetadataResourcesKey] = payload
	}

	envelope, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshalling metadata: %v", err)
	}

	snapshot := &rkev1.ETCDSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "snapshot-1",
			Namespace:   "fleet-default",
			Annotations: map[string]string{},
		},
	}
	snapshot.SnapshotFile.Metadata = base64.StdEncoding.EncodeToString(envelope)
	if availableModes != "" {
		snapshot.Annotations[capr.RestoreModeOptionsAnnotation] = availableModes
	}
	return snapshot
}

// restoredResources is the resources payload for a snapshot taken when the cluster ran an older
// Kubernetes version and a different additional manifest.
func restoredResources() map[string]any {
	return map[string]any{
		rkev1.SnapshotResourceProvCluster: map[string]any{
			"metadata": map[string]any{"name": "test-cluster"},
			"spec": map[string]any{
				"kubernetesVersion": "v1.33.0+rke2r1",
				"rkeConfig": map[string]any{
					"additionalManifest": "# restored",
				},
				// Not in restoremode.WritablePaths: a downstream must not be able to rewrite it.
				"cloudCredentialSecretName": "cattle-global-data:cc-attacker",
			},
		},
	}
}

func kubernetesVersionSelector() string {
	return restoremode.Match{
		ResourceKey: rkev1.SnapshotResourceProvCluster,
		Path:        []string{"spec", "kubernetesVersion"},
	}.String()
}

func TestRestoresClusterConfig(t *testing.T) {
	for _, mode := range []string{"", rkev1.RestoreRKEConfigNone} {
		if restoresClusterConfig(mode) {
			t.Errorf("mode %q should not restore cluster configuration", mode)
		}
	}
	for _, mode := range []string{rkev1.RestoreRKEConfigKubernetesVersion, rkev1.RestoreRKEConfigAll, "customMode"} {
		if !restoresClusterConfig(mode) {
			t.Errorf("mode %q should restore cluster configuration", mode)
		}
	}
}

func TestValidateRestoreMode(t *testing.T) {
	offered := snapshotWithModes(t, nil, nil, "none,kubernetesVersion,all")

	tests := []struct {
		name     string
		mode     string
		snapshot *rkev1.ETCDSnapshot
		ok       bool
		contains string
	}{
		{name: "empty mode needs no snapshot", mode: "", snapshot: nil, ok: true},
		{name: "none needs no snapshot", mode: rkev1.RestoreRKEConfigNone, snapshot: nil, ok: true},
		{
			name:     "a mode with no snapshot CR is rejected",
			mode:     rkev1.RestoreRKEConfigKubernetesVersion,
			snapshot: nil,
			contains: "requires an etcdsnapshot.rke.cattle.io resource",
		},
		{name: "an offered mode is accepted", mode: rkev1.RestoreRKEConfigAll, snapshot: offered, ok: true},
		{
			name:     "a mode the snapshot does not offer is rejected",
			mode:     "customMode",
			snapshot: offered,
			contains: `restore mode "customMode" is not available`,
		},
		{
			name:     "no annotation means no mode is offered",
			mode:     rkev1.RestoreRKEConfigAll,
			snapshot: snapshotWithModes(t, nil, nil, ""),
			contains: "is not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, ok := validateRestoreMode(tt.mode, "snapshot-1", tt.snapshot)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v (reason: %s)", ok, tt.ok, reason)
			}
			if tt.ok {
				if reason != "" {
					t.Errorf("expected no reason, got %q", reason)
				}
				return
			}
			if !strings.Contains(reason, tt.contains) {
				t.Errorf("reason %q does not contain %q", reason, tt.contains)
			}
		})
	}
}

// resolveScope builds a scope whose op requests mode, with adapter serving the given restore
// targets.
func resolveScope(mode string, adapter *stubAdapter) *scope {
	s := newTestScope(adapter, types.UID("uid-1"))
	s.op.Spec.Args.Name = "snapshot-1"
	s.op.Spec.Args.RestoreMode = mode
	return s
}

func TestResolveRestoreMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		modes    map[string]string
		res      map[string]any
		expected []string
		contains string
	}{
		{
			name:     "kubernetesVersion resolves to the captured version",
			mode:     rkev1.RestoreRKEConfigKubernetesVersion,
			modes:    map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: kubernetesVersionSelector()},
			res:      restoredResources(),
			expected: []string{"spec.kubernetesVersion"},
		},
		{
			name:  "all expands to every writable field that was captured",
			mode:  rkev1.RestoreRKEConfigAll,
			modes: map[string]string{rkev1.RestoreRKEConfigAll: rkev1.RestoreModeSelectorWildcard},
			res:   restoredResources(),
			// cloudCredentialSecretName is captured but not writable, so it is dropped.
			expected: []string{"spec.kubernetesVersion", "spec.rkeConfig.additionalManifest"},
		},
		{
			name:     "a mode the payload does not declare is rejected",
			mode:     "customMode",
			modes:    map[string]string{rkev1.RestoreRKEConfigAll: rkev1.RestoreModeSelectorWildcard},
			res:      restoredResources(),
			contains: `does not declare restore mode "customMode"`,
		},
		{
			name:     "a selector naming only non-writable fields is rejected",
			mode:     "credentials",
			modes:    map[string]string{"credentials": restoremode.Match{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: []string{"spec", "cloudCredentialSecretName"}}.String()},
			res:      restoredResources(),
			contains: "selects no field that Rancher restores",
		},
		{
			name:     "an unparsable selector is rejected",
			mode:     rkev1.RestoreRKEConfigKubernetesVersion,
			modes:    map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: "spec.kubernetesVersion"},
			res:      restoredResources(),
			contains: "unusable selector",
		},
		{
			name:     "a selector that resolves to nothing is rejected",
			mode:     rkev1.RestoreRKEConfigKubernetesVersion,
			modes:    map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: kubernetesVersionSelector()},
			res:      map[string]any{},
			contains: "selects no field that Rancher restores",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := snapshotWithModes(t, tt.modes, tt.res, "none,"+tt.mode)
			h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: snapshot}}

			matches, reason, err := h.resolveRestoreMode(resolveScope(tt.mode, defaultAdapter()), tt.mode)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.contains != "" {
				if !strings.Contains(reason, tt.contains) {
					t.Fatalf("reason %q does not contain %q", reason, tt.contains)
				}
				return
			}
			if reason != "" {
				t.Fatalf("unexpected reason: %s", reason)
			}

			var got []string
			for _, m := range matches {
				got = append(got, strings.Join(m.Path, "."))
			}
			assertSameStrings(t, tt.expected, got)
		})
	}

	t.Run("a missing snapshot CR is rejected", func(t *testing.T) {
		h := &handler{etcdsnapshots: &stubSnapshotClient{notFound: true}}

		_, reason, err := h.resolveRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, defaultAdapter()), rkev1.RestoreRKEConfigAll)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(reason, "requires an etcdsnapshot.rke.cattle.io resource") {
			t.Errorf("unexpected reason: %s", reason)
		}
	})

	t.Run("a snapshot with no metadata is rejected", func(t *testing.T) {
		h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: &rkev1.ETCDSnapshot{
			ObjectMeta: metav1.ObjectMeta{Name: "snapshot-1", Namespace: "fleet-default"},
		}}}

		_, reason, err := h.resolveRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, defaultAdapter()), rkev1.RestoreRKEConfigAll)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(reason, "reading metadata") {
			t.Errorf("unexpected reason: %s", reason)
		}
	})
}

func TestApplyRestoreMode(t *testing.T) {
	versionMatch := restoremode.Match{
		ResourceKey: rkev1.SnapshotResourceProvCluster,
		Path:        []string{"spec", "kubernetesVersion"},
		Value:       "v1.33.0+rke2r1",
	}

	t.Run("writes the selected field onto the restore target", func(t *testing.T) {
		adapter := defaultAdapter()
		adapter.restoreTargets = map[string]*unstructured.Unstructured{
			rkev1.SnapshotResourceProvCluster: provClusterTarget(),
		}
		h := &handler{}

		applied, reason, err := h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter), []restoremode.Match{versionMatch})
		if err != nil || reason != "" {
			t.Fatalf("err = %v, reason = %s", err, reason)
		}
		if !applied {
			t.Fatal("expected the restore to be applied")
		}
		if len(adapter.updatedTargets) != 1 {
			t.Fatalf("expected 1 update, got %d", len(adapter.updatedTargets))
		}

		got, found, err := unstructured.NestedString(adapter.updatedTargets[0].Object, "spec", "kubernetesVersion")
		if err != nil || !found {
			t.Fatalf("kubernetesVersion not set: found=%v err=%v", found, err)
		}
		if got != "v1.33.0+rke2r1" {
			t.Errorf("kubernetesVersion = %q, want the captured value", got)
		}

		// Fields the mode did not select are untouched.
		manifest, _, _ := unstructured.NestedString(adapter.updatedTargets[0].Object, "spec", "rkeConfig", "additionalManifest")
		if manifest != "# current" {
			t.Errorf("additionalManifest = %q, want it left alone", manifest)
		}
	})

	t.Run("does not write when the field already holds the captured value", func(t *testing.T) {
		target := provClusterTarget()
		if err := unstructured.SetNestedField(target.Object, "v1.33.0+rke2r1", "spec", "kubernetesVersion"); err != nil {
			t.Fatal(err)
		}

		adapter := defaultAdapter()
		adapter.restoreTargets = map[string]*unstructured.Unstructured{
			rkev1.SnapshotResourceProvCluster: target,
		}
		h := &handler{}

		applied, reason, err := h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter), []restoremode.Match{versionMatch})
		if err != nil || reason != "" {
			t.Fatalf("err = %v, reason = %s", err, reason)
		}
		if applied {
			t.Error("expected no update when the value already matches")
		}
		if len(adapter.updatedTargets) != 0 {
			t.Errorf("expected 0 updates, got %d", len(adapter.updatedTargets))
		}
	})

	t.Run("a cluster with no counterpart for the resource key is rejected", func(t *testing.T) {
		// An imported cluster: ImportedAdapter.RestoreTarget always returns (nil, nil), because
		// nothing upstream holds a configuration to restore.
		adapter := defaultAdapter()
		h := &handler{}

		applied, reason, err := h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter), []restoremode.Match{versionMatch})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if applied {
			t.Error("expected nothing to be applied")
		}
		if !strings.Contains(reason, "which this cluster has no counterpart for") {
			t.Errorf("unexpected reason: %s", reason)
		}
	})

	t.Run("propagates a restore target error", func(t *testing.T) {
		adapter := defaultAdapter()
		adapter.restoreTargetErr = fmt.Errorf("boom")
		h := &handler{}

		if _, _, err := h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, adapter), []restoremode.Match{versionMatch}); err == nil {
			t.Error("expected the error to propagate")
		}
	})

	t.Run("propagates an update error", func(t *testing.T) {
		adapter := defaultAdapter()
		adapter.restoreTargets = map[string]*unstructured.Unstructured{
			rkev1.SnapshotResourceProvCluster: provClusterTarget(),
		}
		adapter.updateRestoreErr = fmt.Errorf("boom")
		h := &handler{}

		if _, _, err := h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, adapter), []restoremode.Match{versionMatch}); err == nil {
			t.Error("expected the error to propagate")
		}
	})
}

func TestReconcileRestoreClusterConfig(t *testing.T) {
	t.Run("a mode restoring nothing advances straight to shutdown", func(t *testing.T) {
		adapter := defaultAdapter()
		h := &handler{}
		s := resolveScope(rkev1.RestoreRKEConfigNone, adapter)

		status, err := h.reconcileRestoreClusterConfig(s, opv1alpha1.ETCDSnapshotRestoreStatus{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Step != opv1alpha1.ETCDSnapshotRestoreStepShutdown {
			t.Errorf("step = %q, want Shutdown", status.Step)
		}
		if len(adapter.restoreTargetKeys) != 0 {
			t.Errorf("expected no restore target lookups, got %v", adapter.restoreTargetKeys)
		}
	})

	t.Run("applies the mode then advances on the next reconcile", func(t *testing.T) {
		adapter := defaultAdapter()
		adapter.restoreTargetSettled = true
		adapter.restoreTargets = map[string]*unstructured.Unstructured{
			rkev1.SnapshotResourceProvCluster: provClusterTarget(),
		}
		snapshot := snapshotWithModes(t,
			map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: kubernetesVersionSelector()},
			restoredResources(),
			"none,kubernetesVersion")
		h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: snapshot}}
		s := resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter)

		// First pass writes and stays in the step, because the caches this reconcile read are now
		// stale.
		status, err := h.reconcileRestoreClusterConfig(s, opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Step != opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig {
			t.Errorf("step = %q, want to stay in RestoreClusterConfig", status.Step)
		}
		if len(adapter.updatedTargets) != 1 {
			t.Fatalf("expected 1 update, got %d", len(adapter.updatedTargets))
		}

		// Second pass sees the applied value and advances without writing again.
		status, err = h.reconcileRestoreClusterConfig(s, status)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Step != opv1alpha1.ETCDSnapshotRestoreStepShutdown {
			t.Errorf("step = %q, want Shutdown", status.Step)
		}
		if len(adapter.updatedTargets) != 1 {
			t.Errorf("expected no second update, got %d", len(adapter.updatedTargets))
		}
	})

	t.Run("holds until the cluster has observed the restored configuration", func(t *testing.T) {
		// The write has already landed, so there is nothing left to apply — but the objects rendered
		// off the restore target have not caught up. Advancing here would let the Restore step build
		// a node plan, and install a Kubernetes version, from the pre-restore configuration.
		target := provClusterTarget()
		if err := unstructured.SetNestedField(target.Object, "v1.33.0+rke2r1", "spec", "kubernetesVersion"); err != nil {
			t.Fatal(err)
		}

		adapter := defaultAdapter()
		adapter.restoreTargetSettled = false
		adapter.restoreTargets = map[string]*unstructured.Unstructured{
			rkev1.SnapshotResourceProvCluster: target,
		}
		snapshot := snapshotWithModes(t,
			map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: kubernetesVersionSelector()},
			restoredResources(),
			"none,kubernetesVersion")
		h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: snapshot}}
		s := resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter)

		status, err := h.reconcileRestoreClusterConfig(s, opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Step != opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig {
			t.Errorf("step = %q, want to stay in RestoreClusterConfig", status.Step)
		}
		if len(adapter.updatedTargets) != 0 {
			t.Errorf("expected no writes, got %d", len(adapter.updatedTargets))
		}

		// Once it settles, the same reconcile advances without writing.
		adapter.restoreTargetSettled = true
		status, err = h.reconcileRestoreClusterConfig(s, status)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Step != opv1alpha1.ETCDSnapshotRestoreStepShutdown {
			t.Errorf("step = %q, want Shutdown", status.Step)
		}
	})

	t.Run("propagates a restore target wait error", func(t *testing.T) {
		target := provClusterTarget()
		if err := unstructured.SetNestedField(target.Object, "v1.33.0+rke2r1", "spec", "kubernetesVersion"); err != nil {
			t.Fatal(err)
		}

		adapter := defaultAdapter()
		adapter.restoreTargets = map[string]*unstructured.Unstructured{
			rkev1.SnapshotResourceProvCluster: target,
		}
		adapter.waitRestoreTargetErr = fmt.Errorf("boom")
		snapshot := snapshotWithModes(t,
			map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: kubernetesVersionSelector()},
			restoredResources(),
			"none,kubernetesVersion")
		h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: snapshot}}

		_, err := h.reconcileRestoreClusterConfig(resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter), opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err == nil {
			t.Error("expected the error to propagate")
		}
	})

	t.Run("a mode restoring nothing still does not wait", func(t *testing.T) {
		// "none" writes nothing, so there is nothing to propagate and no reason to hold.
		adapter := defaultAdapter()
		adapter.restoreTargetSettled = false
		h := &handler{}

		status, err := h.reconcileRestoreClusterConfig(resolveScope("", adapter), opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Step != opv1alpha1.ETCDSnapshotRestoreStepShutdown {
			t.Errorf("step = %q, want Shutdown", status.Step)
		}
	})

	t.Run("cancels when the mode cannot be resolved", func(t *testing.T) {
		adapter := defaultAdapter()
		h := &handler{etcdsnapshots: &stubSnapshotClient{notFound: true}}
		s := resolveScope(rkev1.RestoreRKEConfigAll, adapter)

		status, err := h.reconcileRestoreClusterConfig(s, opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Phase != opv1alpha1.OperationPhaseCanceled {
			t.Errorf("phase = %q, want Canceled", status.Phase)
		}
	})

	t.Run("cancels rather than silently restoring nothing", func(t *testing.T) {
		// The mode resolves, but this cluster type has no object to write it to. Degrading to a
		// plain etcd restore would give the user something they did not ask for.
		adapter := defaultAdapter()
		snapshot := snapshotWithModes(t,
			map[string]string{rkev1.RestoreRKEConfigKubernetesVersion: kubernetesVersionSelector()},
			restoredResources(),
			"none,kubernetesVersion")
		h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: snapshot}}
		s := resolveScope(rkev1.RestoreRKEConfigKubernetesVersion, adapter)

		status, err := h.reconcileRestoreClusterConfig(s, opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Phase != opv1alpha1.OperationPhaseCanceled {
			t.Errorf("phase = %q, want Canceled", status.Phase)
		}
	})

}

// TestStepHookPrefixForCoversEveryStep guards the wiring a new step needs: a step missing from
// stepHookPrefixFor silently loses its lifecycle hook, and handleInProgress's beacon-loss handling
// consults the prefix to tell an intentional delegation from a genuine loss.
func TestStepHookPrefixForCoversEveryStep(t *testing.T) {
	t.Parallel()

	steps := []opv1alpha1.ETCDSnapshotRestoreStep{
		opv1alpha1.ETCDSnapshotRestoreStepPreflight,
		opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		opv1alpha1.ETCDSnapshotRestoreStepShutdown,
		opv1alpha1.ETCDSnapshotRestoreStepRestore,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepInitialRestartCluster,
		opv1alpha1.ETCDSnapshotRestoreStepPostRestoreNodeCleanup,
		opv1alpha1.ETCDSnapshotRestoreStepRestartCluster,
	}

	seen := map[string]opv1alpha1.ETCDSnapshotRestoreStep{}
	for _, step := range steps {
		prefix := stepHookPrefixFor(step)
		if prefix == "" {
			t.Errorf("step %q has no hook prefix", step)
			continue
		}
		if other, ok := seen[prefix]; ok {
			t.Errorf("steps %q and %q share hook prefix %q", step, other, prefix)
		}
		seen[prefix] = step
	}
}

func assertSameStrings(t *testing.T, want, got []string) {
	t.Helper()

	if len(want) != len(got) {
		t.Fatalf("got %v, want %v", got, want)
	}
	remaining := map[string]int{}
	for _, w := range want {
		remaining[w]++
	}
	for _, g := range got {
		remaining[g]--
	}
	for k, count := range remaining {
		if count != 0 {
			t.Fatalf("got %v, want %v (mismatch on %q)", got, want, k)
		}
	}
}

// TestBuildRestorePlanInstallsConfiguredVersion covers the downgrade mechanism: the restore plan has
// to reinstall the distro at the configured Kubernetes version before --cluster-reset, because a
// newer server cannot reset onto etcd data written by an older one.
func TestBuildRestorePlanInstallsConfiguredVersion(t *testing.T) {
	t.Parallel()

	secret := makePlanSecret("etcd-0", "node-etcd-0", map[string]string{capr.EtcdRoleLabel: "true"})

	t.Run("the install precedes the etcd wipe and the reset", func(t *testing.T) {
		t.Parallel()

		adapter := defaultAdapter()
		adapter.installVersion = "v1.33.0+rke2r1"
		s := newTestScope(adapter, types.UID("uid-1"))

		nodePlan := buildRestorePlan(s, secret, nil, "snapshot-1")

		assertInstructionOrder(t, nodePlan, []string{idempotencyKey + "/install", idempotencyKey + "/clean-etcd-dir", idempotencyKey + "/restore"})

		// The install has to carry the version-tagged image and must not start the distro: the
		// restore itself is what brings the server up, via --cluster-reset.
		install := nodePlan.OneTimeInstructions[0]
		if install.Image == "" || !strings.HasSuffix(install.Image, ":v1.33.0-rke2r1") {
			t.Errorf("image = %q, want it tagged with the configured version", install.Image)
		}
		if !slices.Contains(install.Env, "INSTALL_RKE2_SKIP_START=true") {
			t.Errorf("env = %v, want the distro start suppressed", install.Env)
		}
	})

	t.Run("a cluster type that does not manage its version restores without reinstalling", func(t *testing.T) {
		t.Parallel()

		// An imported cluster: Rancher does not choose its distro version, so there is nothing to
		// install and the restore proceeds as it did before this step existed.
		adapter := defaultAdapter()
		s := newTestScope(adapter, types.UID("uid-1"))

		nodePlan := buildRestorePlan(s, secret, nil, "snapshot-1")

		assertInstructionOrder(t, nodePlan, []string{idempotencyKey + "/clean-etcd-dir", idempotencyKey + "/restore"})
	})

	t.Run("the install is scoped to this operation", func(t *testing.T) {
		t.Parallel()

		// Every restore instruction runs through the idempotency wrapper keyed on the op's UID, so a
		// re-reconcile does not reinstall and two operations never share tracking state.
		adapter := defaultAdapter()
		adapter.installVersion = "v1.33.0+rke2r1"

		first := buildRestorePlan(newTestScope(adapter, types.UID("uid-1")), secret, nil, "snapshot-1")
		second := buildRestorePlan(newTestScope(adapter, types.UID("uid-2")), secret, nil, "snapshot-1")

		if fmt.Sprint(first.OneTimeInstructions[0].Args) == fmt.Sprint(second.OneTimeInstructions[0].Args) {
			t.Error("expected the install instruction to be scoped to the operation UID")
		}
		if !slices.Contains(first.OneTimeInstructions[0].Args, idempotencyKey+"/install") {
			t.Errorf("args = %v, want the install idempotency key", first.OneTimeInstructions[0].Args)
		}
	})
}

// assertInstructionOrder checks the plan's one-time instructions carry the given idempotency
// identifiers in order.
func assertInstructionOrder(t *testing.T, nodePlan *planapi.Plan, want []string) {
	t.Helper()

	var got []string
	for _, inst := range nodePlan.OneTimeInstructions {
		got = append(got, instructionIdentifier(inst))
	}

	if len(got) != len(want) {
		t.Fatalf("instructions = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("instruction %d = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}

// instructionIdentifier returns the idempotency identifier of a wrapped instruction.
// ops.IdempotentInstruction rewrites the instruction to run through the idempotency script with the
// argument list [-x, <script>, <identifier>, <hashedValue>, <hashedCommand>, <command>,
// <provisioningDir>, <args>...], so the identifier is the third argument.
func instructionIdentifier(inst planapi.OneTimeInstruction) string {
	if inst.Command != "/bin/sh" || len(inst.Args) < 3 {
		return inst.Name
	}
	return inst.Args[2]
}
