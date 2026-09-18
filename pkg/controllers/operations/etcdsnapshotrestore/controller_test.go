package etcdsnapshotrestore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/generic"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(testClusterGVK)
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test-cluster")
	return cluster, nil
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

type fakeETCDSnapshotRestoreController struct {
	operationcontrollers.ETCDSnapshotRestoreController
	enqueueCalls int
	deleteCalls  int
	// updates records the objects passed to Update — the finalizer is the only thing the handler
	// writes outside of status, so each entry is a finalizer add or removal.
	updates []*opv1alpha1.ETCDSnapshotRestore
}

func (f *fakeETCDSnapshotRestoreController) EnqueueAfter(_, _ string, _ time.Duration) {
	f.enqueueCalls++
}

func (f *fakeETCDSnapshotRestoreController) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	f.deleteCalls++
	return nil
}

func (f *fakeETCDSnapshotRestoreController) Update(op *opv1alpha1.ETCDSnapshotRestore) (*opv1alpha1.ETCDSnapshotRestore, error) {
	f.updates = append(f.updates, op.DeepCopy())
	return op, nil
}

// fakeBeaconClient is a tiny in-memory BeaconClient covering only the methods the handler reaches
// for: Get during scope resolution, and Update/UpdateStatus from the Acquire/Release/Push helpers.
// beacon is kept in step with UpdateStatus so a handler driven over several reconciles observes its
// own beacon writes; a nil beacon makes Get report NotFound.
type fakeBeaconClient struct {
	plancontrollers.BeaconClient // embed for any unused method; nil panics signal an unexpected call

	beacon *planv1alpha1.Beacon

	updates       []*planv1alpha1.Beacon
	statusUpdates []*planv1alpha1.Beacon
}

func (f *fakeBeaconClient) Get(namespace, name string, _ metav1.GetOptions) (*planv1alpha1.Beacon, error) {
	if f.beacon == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "beacons"}, namespace+"/"+name)
	}
	return f.beacon.DeepCopy(), nil
}

func (f *fakeBeaconClient) Update(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.updates = append(f.updates, b.DeepCopy())
	return b, nil
}

func (f *fakeBeaconClient) UpdateStatus(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, b.DeepCopy())
	f.beacon = b.DeepCopy()
	return b, nil
}

// fakeDynamic satisfies the controller's dynamicResolver interface. Enqueue records the
// (gvk, namespace, name) tuple so tests can assert handleSucceeded nudged the parent cluster.
type fakeDynamic struct {
	gets     map[string]runtime.Object
	enqueued []string
}

func (f *fakeDynamic) Get(gvk schema.GroupVersionKind, ns, name string) (runtime.Object, error) {
	if obj, ok := f.gets[gvk.String()+"/"+ns+"/"+name]; ok {
		return obj, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
}

func (f *fakeDynamic) Enqueue(gvk schema.GroupVersionKind, ns, name string) error {
	f.enqueued = append(f.enqueued, gvk.String()+"/"+ns+"/"+name)
	return nil
}

// testClusterGVK is a synthetic cluster kind registered with the ops adapter factory below, so the
// OnChange tests can drive a reconcile end to end — cluster lookup, adapter, beacon lookup, phase
// dispatch — without standing up a real provisioning/CAPI cluster. Using a dedicated kind also
// keeps the registration from shadowing the adapter of a kind that ships with Rancher.
var testClusterGVK = schema.GroupVersionKind{Group: "test.cattle.io", Version: "v1", Kind: "TestCluster"}

func init() {
	ops.RegisterAdapter(testClusterGVK, func(_ *wrangler.CAPIContext, _ *unstructured.Unstructured) (ops.Adapter, error) {
		return defaultAdapter(), nil
	})
}

// newDeletingOp returns an operation which has been deleted and still carries our finalizer — the
// state the API server leaves an in-flight operation in until the controller releases it.
func newDeletingOp() *opv1alpha1.ETCDSnapshotRestore {
	op := newOp()
	op.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	op.Finalizers = []string{Finalizer}
	return op
}

// testOwnerKey is the fully-qualified beacon owner key for the canonical newOp() operation, so
// beacons built with it match the ownership and delegate-chain checks the handler performs.
var testOwnerKey = planapi.ControllerOwnerKey(newOp(), ControllerOwnerKey)

func newBeacon(owner string, active bool) *planv1alpha1.Beacon {
	return &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "test-namespace",
		},
		Status: planv1alpha1.BeaconStatus{
			Active: active,
			Owner:  owner,
		},
	}
}

// newScope wires the per-reconcile context for the terminal-handler tests, with the ownerKey the
// real controller computes so beacon fixtures created with testOwnerKey are recognised as ours.
func newScope(op *opv1alpha1.ETCDSnapshotRestore, beacon *planv1alpha1.Beacon) *scope {
	cluster, _ := defaultAdapter().ClusterObject()
	return &scope{
		ownerKey:   planapi.ControllerOwnerKey(op, ControllerOwnerKey),
		op:         op,
		beacon:     beacon,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    defaultAdapter(),
	}
}

// newOnChangeHandler wires a handler for the full OnChange path: the cluster referenced by newOp()
// resolves through the dynamic resolver, and the beacon lookup serves (and mutates) the given
// beacon. A nil beacon makes the lookup report NotFound.
func newOnChangeHandler(beacon *planv1alpha1.Beacon) (*handler, *fakeETCDSnapshotRestoreController, *fakeBeaconClient) {
	controller := &fakeETCDSnapshotRestoreController{}
	beacons := &fakeBeaconClient{beacon: beacon}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(testClusterGVK)
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test-cluster")

	h := &handler{
		etcdsnapshotrestores: controller,
		beacons:              beacons,
		dynamic: &fakeDynamic{gets: map[string]runtime.Object{
			testClusterGVK.String() + "/fleet-default/test-cluster": cluster,
		}},
	}

	return h, controller, beacons
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

// newOp is the canonical operation fixture. Its ClusterRef points at testClusterGVK so the tests
// that drive OnChange end to end resolve through newOnChangeHandler's fake dynamic resolver; tests
// that stop before scope resolution (paused, terminal handlers) simply never consult it.
func newOp() *opv1alpha1.ETCDSnapshotRestore {
	return &opv1alpha1.ETCDSnapshotRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "restore-1",
			Namespace:  "fleet-default",
			UID:        types.UID("restore-uid"),
			Generation: 1,
		},
		Spec: opv1alpha1.ETCDSnapshotRestoreSpec{
			OperationSpec: opv1alpha1.OperationSpec{
				ClusterRef: &corev1.ObjectReference{
					APIVersion: testClusterGVK.GroupVersion().String(),
					Kind:       testClusterGVK.Kind,
					Namespace:  "fleet-default",
					Name:       "test-cluster",
				},
			},
		},
	}
}

func TestUpdateStatusPausedCondition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name            string
		paused          bool
		initiallyPaused bool
		expectedStatus  string
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "paused",
			paused:          true,
			initiallyPaused: false,
			expectedStatus:  "True",
			expectedReason:  opv1alpha1.PausedReason,
			expectedMessage: "Operation is paused",
		},
		{
			name:            "resumed",
			paused:          false,
			initiallyPaused: true,
			expectedStatus:  "False",
			expectedReason:  opv1alpha1.NotPausedReason,
			expectedMessage: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := newOp()
			op.Spec.Paused = tc.paused
			op.Generation = 7

			initialStatus := opv1alpha1.ETCDSnapshotRestoreStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase: opv1alpha1.OperationPhaseInProgress,
				},
				Step: opv1alpha1.ETCDSnapshotRestoreStepRestore,
			}

			if tc.initiallyPaused {
				opv1alpha1.PausedCondition.True(&initialStatus)
				opv1alpha1.PausedCondition.Reason(&initialStatus, opv1alpha1.PausedReason)
				opv1alpha1.PausedCondition.Message(&initialStatus, "Operation is paused")
			}

			status := updateStatus(op, initialStatus)

			if status.ObservedGeneration != int64(7) {
				t.Errorf("ObservedGeneration = %d, want 7", status.ObservedGeneration)
			}
			if got := opv1alpha1.PausedCondition.GetStatus(&status); got != tc.expectedStatus {
				t.Errorf("PausedCondition status = %q, want %q", got, tc.expectedStatus)
			}
			if got := opv1alpha1.PausedCondition.GetReason(&status); got != tc.expectedReason {
				t.Errorf("PausedCondition reason = %q, want %q", got, tc.expectedReason)
			}
			if got := opv1alpha1.PausedCondition.GetMessage(&status); got != tc.expectedMessage {
				t.Errorf("PausedCondition message = %q, want %q", got, tc.expectedMessage)
			}

			// Verify phase and step are unchanged
			if status.Phase != initialStatus.Phase {
				t.Errorf("Phase = %q, want %q (unchanged)", status.Phase, initialStatus.Phase)
			}
			if status.Step != initialStatus.Step {
				t.Errorf("Step = %q, want %q (unchanged)", status.Step, initialStatus.Step)
			}
		})
	}
}

func TestOnChange_StablePausedOperation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		phase       opv1alpha1.OperationPhase
		step        opv1alpha1.ETCDSnapshotRestoreStep
		ttl         int64
		lastUpdated metav1.Time
	}{
		{
			name:        "stable in-progress paused operation",
			phase:       opv1alpha1.OperationPhaseInProgress,
			step:        opv1alpha1.ETCDSnapshotRestoreStepRestore,
			ttl:         300,
			lastUpdated: metav1.Now(),
		},
		{
			name:        "stable terminal expired paused operation",
			phase:       opv1alpha1.OperationPhaseSucceeded,
			step:        opv1alpha1.ETCDSnapshotRestoreStepRestore,
			ttl:         0,
			lastUpdated: metav1.NewTime(metav1.Now().Add(-10 * time.Minute)),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := newOp()
			op.Spec.Paused = true
			op.Spec.TTL = tc.ttl
			op.Generation = 7

			initialStatus := opv1alpha1.ETCDSnapshotRestoreStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase:       tc.phase,
					LastUpdated: tc.lastUpdated,
				},
				Step: tc.step,
			}

			// Pre-compute the expected status with paused condition
			currentStatus := updateStatus(op, initialStatus)
			op.Status = currentStatus

			controller := &fakeETCDSnapshotRestoreController{}
			h := &handler{
				etcdsnapshotrestores: controller,
			}

			returnedStatus, err := h.OnChange(op, op.Status)
			if err != nil {
				t.Fatalf("OnChange returned error: %v", err)
			}

			// Verify status unchanged
			if !reflect.DeepEqual(returnedStatus, currentStatus) {
				t.Errorf("returnedStatus differs from currentStatus")
			}

			// Verify phase and step preserved
			if returnedStatus.Phase != tc.phase {
				t.Errorf("Phase = %q, want %q", returnedStatus.Phase, tc.phase)
			}
			if returnedStatus.Step != tc.step {
				t.Errorf("Step = %q, want %q", returnedStatus.Step, tc.step)
			}

			// Verify no delete occurred
			if controller.deleteCalls != 0 {
				t.Errorf("Delete called %d times, want 0", controller.deleteCalls)
			}

			// Verify no enqueue occurred
			if controller.enqueueCalls != 0 {
				t.Errorf("EnqueueAfter called %d times, want 0", controller.enqueueCalls)
			}
		})
	}
}

func TestOnChange_Paused(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true
	op.Generation = 7

	initialStatus := opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase: opv1alpha1.OperationPhaseInProgress,
		},
		Step: opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	op.Status = initialStatus

	h := &handler{}
	status, err := h.OnChange(op, op.Status)

	if err != nil {
		t.Fatalf("OnChange returned error: %v", err)
	}
	if got := opv1alpha1.PausedCondition.GetStatus(&status); got != "True" {
		t.Errorf("PausedCondition status = %q, want %q", got, "True")
	}
	if got := opv1alpha1.PausedCondition.GetReason(&status); got != opv1alpha1.PausedReason {
		t.Errorf("PausedCondition reason = %q, want %q", got, opv1alpha1.PausedReason)
	}
	if got := opv1alpha1.PausedCondition.GetMessage(&status); got != "Operation is paused" {
		t.Errorf("PausedCondition message = %q, want %q", got, "Operation is paused")
	}
	if status.ObservedGeneration != int64(7) {
		t.Errorf("ObservedGeneration = %d, want 7", status.ObservedGeneration)
	}
	if status.Phase != initialStatus.Phase {
		t.Errorf("Phase = %q, want %q (unchanged)", status.Phase, initialStatus.Phase)
	}
	if status.Step != initialStatus.Step {
		t.Errorf("Step = %q, want %q (unchanged)", status.Step, initialStatus.Step)
	}
}

// --- terminal handling -----------------------------------------------------------------------

// terminalHandlers enumerates the three terminal phase handlers along with the condition each one
// reports through and the lifecycle-hook prefix that delegates it, so the tests below can assert
// the shared termination-recording contract once for all of them.
var terminalHandlers = map[string]struct {
	handle func(*handler, *scope, opv1alpha1.ETCDSnapshotRestoreStatus) (opv1alpha1.ETCDSnapshotRestoreStatus, error)
	cond   condition.Cond
	hook   string
}{
	"canceled": {
		handle: (*handler).handleCanceled,
		cond:   opv1alpha1.CanceledCondition,
		hook:   planv1alpha1.CanceledPhaseHookLabelPrefix,
	},
	"failed": {
		handle: (*handler).handleFailed,
		cond:   opv1alpha1.FailedCondition,
		hook:   planv1alpha1.FailedPhaseHookLabelPrefix,
	},
	"succeeded": {
		handle: (*handler).handleSucceeded,
		cond:   opv1alpha1.SucceededCondition,
		hook:   planv1alpha1.SucceededPhaseHookLabelPrefix,
	},
}

func TestHandleTerminal_RecordsTermination(t *testing.T) {
	t.Parallel()

	for name, tc := range terminalHandlers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			beacons := &fakeBeaconClient{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(newOp(), newBeacon(testOwnerKey, true))

			got, err := tc.handle(h, s, opv1alpha1.ETCDSnapshotRestoreStatus{})
			assert.NoError(t, err)
			assert.False(t, got.TerminatedAt.IsZero(),
				"terminal handling completed (beacon released), so it must be recorded on the status")
			if assert.Len(t, beacons.statusUpdates, 1, "the beacon must be released") {
				assert.Equal(t, "", beacons.statusUpdates[0].Status.Owner)
				assert.False(t, beacons.statusUpdates[0].Status.Active)
			}
		})
	}
}

// TestHandleTerminal_DelegatedDefersTermination is the counterpart: while a terminal phase hook is
// still delegated the handler has not finished, the beacon is still held on the operation's behalf,
// and nothing may be recorded — that marker is what releases the operation for deletion and for TTL
// garbage collection.
func TestHandleTerminal_DelegatedDefersTermination(t *testing.T) {
	t.Parallel()

	for name, tc := range terminalHandlers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			op := newOp()
			op.Labels = map[string]string{tc.hook + "test": "delegate-a"}

			beacons := &fakeBeaconClient{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(op, newBeacon(testOwnerKey, true))

			// The outcome the phase handler recorded before delegating. It is the only record of
			// why the operation ended, so delegating must not overwrite it — the delegate is
			// reported on Finalized by updateStatus instead.
			status := opv1alpha1.ETCDSnapshotRestoreStatus{}
			tc.cond.True(&status)
			tc.cond.Reason(&status, opv1alpha1.PlanFailedReason)
			tc.cond.Message(&status, "the operative detail")

			got, err := tc.handle(h, s, status)
			assert.NoError(t, err)
			assert.Equal(t, opv1alpha1.PlanFailedReason, tc.cond.GetReason(&got),
				"the outcome reason must survive the delegation")
			assert.Equal(t, "the operative detail", tc.cond.GetMessage(&got))
			assert.True(t, got.TerminatedAt.IsZero(),
				"terminal handling is still delegated, so it must not be recorded as complete")
			if assert.Len(t, beacons.statusUpdates, 1, "the hook must push its delegate onto the beacon") {
				assert.Equal(t, testOwnerKey, beacons.statusUpdates[0].Status.Owner,
					"the beacon must still be held while the delegate works")
				assert.Contains(t, beacons.statusUpdates[0].Status.Delegates, "delegate-a")
			}
		})
	}
}

// --- deletion --------------------------------------------------------------------------------

func TestOnChange_DeletingWithoutOurFinalizerIsSkipped(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Finalizers = nil

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	_, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Empty(t, controller.updates, "an operation we never finalized has no teardown left to run")
	assert.Empty(t, beacons.statusUpdates, "the beacon must not be touched")
	assert.Zero(t, controller.enqueueCalls, "an operation deleting under someone else's finalizer must not be polled")
}

// TestOnChange_DeletionCancelsInFlightOperation covers the core of the deletion contract: an
// operation deleted while it is still running is canceled, its beacon is released, and only then is
// the finalizer retired — one reconcile later, so the canceled status is persisted for observers
// before the object is allowed to disappear.
//
// A restore is the operation where this matters most: it is deleted mid-flight with the cluster's
// server units shut down, so whoever picks up the pieces needs the beacon back and a recorded
// reason for why nothing is driving the restore any more.
func TestOnChange_DeletionCancelsInFlightOperation(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.Equal(t, opv1alpha1.OperationDeletedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.False(t, status.TerminatedAt.IsZero(), "handleCanceled ran to completion, so it must be recorded")
	if assert.Len(t, beacons.statusUpdates, 1, "the beacon must be released on the way out") {
		assert.Equal(t, "", beacons.statusUpdates[0].Status.Owner)
		assert.False(t, beacons.statusUpdates[0].Status.Active)
	}
	assert.Empty(t, controller.updates, "the finalizer must outlive the status write that records the cancellation")

	// Second pass, with the status the first pass returned now persisted: nothing moves, so the
	// finalizer can go and the API server can finish the deletion.
	op.Status = status

	status, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase, "the recorded outcome must not change")
	if assert.Len(t, controller.updates, 1, "the finalizer must be removed once terminal handling is recorded") {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
	assert.Zero(t, controller.deleteCalls, "the object is already deleting; TTL cleanup must not fire")
}

// TestOnChange_DeletionWaitsForTerminalHook is the case the finalizer exists for: the operation is
// deleted while a canceled phase hook delegate holds the beacon on its behalf. The operation must
// be kept alive — the delegate's ownership is anchored to it — until the delegate is done.
func TestOnChange_DeletionWaitsForTerminalHook(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Labels = map[string]string{planv1alpha1.CanceledPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	// The cancellation reason is the operation's outcome and must not be displaced by the delegate;
	// the delegate is reported on Finalized, which is the thing still outstanding.
	assert.Equal(t, opv1alpha1.OperationDeletedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.Equal(t, "False", opv1alpha1.FinalizedCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.FinalizedCondition.GetReason(&status))
	assert.Contains(t, opv1alpha1.FinalizedCondition.GetMessage(&status), "delegate-a")
	assert.True(t, status.TerminatedAt.IsZero())
	assert.Empty(t, controller.updates)

	// Stable pass while the delegate works: the operation is polled, not released.
	op.Status = status

	status, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.True(t, status.TerminatedAt.IsZero())
	assert.Empty(t, controller.updates, "the finalizer must be held until the delegate hands the beacon back")
	assert.Equal(t, 1, controller.enqueueCalls, "the operation must keep polling for the delegate to finish")

	// The delegate finishes and drops its hook label; terminal handling can now complete.
	op.Labels = nil

	status, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.False(t, status.TerminatedAt.IsZero(), "the beacon has been released, so termination is recorded")
	assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&status), "the delegate is gone, so the wait must resolve")
	assert.Equal(t, opv1alpha1.FinishedReason, opv1alpha1.FinalizedCondition.GetReason(&status))
	assert.Equal(t, opv1alpha1.OperationDeletedReason, opv1alpha1.CanceledCondition.GetReason(&status),
		"the outcome reason must still be the one recorded at cancellation")
	assert.NotEmpty(t, beacons.statusUpdates)
	assert.Empty(t, controller.updates, "the terminal status is persisted before the finalizer is dropped")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1) {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_DeletionPreservesTerminatedOutcome guards the other half of the rule: cancellation is
// only for operations deleted *before* their terminal handling completed. An operation which
// finished its work keeps the phase it finished in.
func TestOnChange_DeletionPreservesTerminatedOutcome(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Status = updateStatus(op, opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:        opv1alpha1.OperationPhaseSucceeded,
			TerminatedAt: metav1.Now(),
		},
		Step: opv1alpha1.ETCDSnapshotRestoreStepRestartCluster,
	})

	h, controller, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase, "a terminated operation must not be demoted to Canceled")
	assert.Equal(t, "True", opv1alpha1.SucceededCondition.GetStatus(&status), "the outcome it finished with must be asserted")
	assert.Equal(t, "False", opv1alpha1.CanceledCondition.GetStatus(&status), "the Canceled condition must be denied, not raised")
	if assert.Len(t, controller.updates, 1, "terminal handling was already complete, so the finalizer can go immediately") {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_DeletionOfPausedOperation covers a paused operation being deleted: pausing halts
// execution, but it must not wedge a deletion behind the finalizer.
func TestOnChange_DeletionOfPausedOperation(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Spec.Paused = true
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepShutdown,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.NotEmpty(t, beacons.statusUpdates, "the beacon must be released even though the operation is paused")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1, "a paused operation must still be releasable for deletion") {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_DeletionWithMissingCluster covers deleting an operation whose cluster (and with it
// the beacon it refers to) is already gone. There is nothing left to release, so the operation must
// not sit in Terminating waiting for a cluster that will never come back.
func TestOnChange_DeletionWithMissingCluster(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Spec.ClusterRef.Name = "gone"
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	h, controller, _ := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase, "the operation never finished, so it is canceled")
	assert.False(t, status.TerminatedAt.IsZero(), "with no cluster there is nothing to release")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1) {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_DeletionWithMissingBeacon is the same situation one step further in: the cluster is
// still around but its beacon has already been collected.
func TestOnChange_DeletionWithMissingBeacon(t *testing.T) {
	t.Parallel()

	op := newDeletingOp()
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	h, controller, _ := newOnChangeHandler(nil)

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.False(t, status.TerminatedAt.IsZero(), "with no beacon there is nothing to release")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1) {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// --- finalizer -------------------------------------------------------------------------------

func TestOnChange_TakesFinalizer(t *testing.T) {
	t.Parallel()

	// The operation starts out Pending, which keeps the reconcile in handlePending (waiting for
	// system-agents to register) rather than dispatching plans the fake has no secrets for.
	op := newOp()

	h, controller, _ := newOnChangeHandler(newBeacon(testOwnerKey, true))

	_, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1, "the finalizer has to be written with an explicit Update") {
		assert.Contains(t, controller.updates[0].Finalizers, Finalizer)
	}
	assert.Contains(t, op.Finalizers, Finalizer,
		"the object the status handler goes on to update must reflect the write, or its UpdateStatus conflicts")

	// Subsequent reconciles must not keep writing it.
	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Len(t, controller.updates, 1, "taking the finalizer must be idempotent")
}

// TestOnChange_PausedOperationDoesNotTakeFinalizer documents the deliberate exception: a paused
// operation has dispatched nothing since it was paused, so taking the finalizer would only stand
// between the user and deleting it.
func TestOnChange_PausedOperationDoesNotTakeFinalizer(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true

	h, controller, _ := newOnChangeHandler(newBeacon(testOwnerKey, true))

	_, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Empty(t, controller.updates)
	assert.Empty(t, op.Finalizers)
}

// --- TTL garbage collection ------------------------------------------------------------------

// TestOnChange_ExpiredTerminalOperationIsCollectedOnceTerminated pins the TTL guard to the same
// marker the deletion flow uses. Collecting an operation whose terminal handling had not completed
// would strand whatever still holds its beacon, and the deletion it triggers would then rewrite the
// phase it finished in to Canceled.
func TestOnChange_ExpiredTerminalOperationIsCollectedOnceTerminated(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Finalizers = []string{Finalizer}
	op.Spec.TTL = 0 // expire as soon as the operation is terminal
	op.Labels = map[string]string{planv1alpha1.SucceededPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:       opv1alpha1.OperationPhaseSucceeded,
			LastUpdated: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Step: opv1alpha1.ETCDSnapshotRestoreStepRestartCluster,
	}

	h, controller, _ := newOnChangeHandler(newBeacon(testOwnerKey, true))

	// handleSucceeded is delegated, so termination is not recorded and the expired operation stays.
	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.True(t, status.TerminatedAt.IsZero())

	op.Status = status

	status, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Zero(t, controller.deleteCalls, "an operation whose terminal handling is unfinished must not be collected")
	assert.Equal(t, 1, controller.enqueueCalls)

	// The delegate finishes: handleSucceeded releases the beacon and records termination.
	op.Labels = nil

	status, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.False(t, status.TerminatedAt.IsZero())
	assert.Zero(t, controller.deleteCalls, "the status moved this pass, so collection waits for it to settle")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.ErrorIs(t, err, generic.ErrSkip, "the collected operation must not be processed further")
	assert.Equal(t, 1, controller.deleteCalls)
}

// --- conditions ------------------------------------------------------------------------------

// TestUpdateStatusTerminatedOutcome covers the other half of the outcome contract: once terminal
// handling is recorded, the matching outcome condition goes True while keeping the reason the phase
// handler gave it, the competing outcomes go False, Finalized summarises all three, and the progress
// conditions are cleared.
func TestUpdateStatusTerminatedOutcome(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		phase   opv1alpha1.OperationPhase
		reason  string
		outcome condition.Cond
		others  []condition.Cond
	}{
		{
			name:    "succeeded",
			phase:   opv1alpha1.OperationPhaseSucceeded,
			reason:  opv1alpha1.FinishedReason,
			outcome: opv1alpha1.SucceededCondition,
			others:  []condition.Cond{opv1alpha1.FailedCondition, opv1alpha1.CanceledCondition},
		},
		{
			name:    "failed",
			phase:   opv1alpha1.OperationPhaseFailed,
			reason:  opv1alpha1.PlanFailedReason,
			outcome: opv1alpha1.FailedCondition,
			others:  []condition.Cond{opv1alpha1.SucceededCondition, opv1alpha1.CanceledCondition},
		},
		{
			name:    "canceled",
			phase:   opv1alpha1.OperationPhaseCanceled,
			reason:  opv1alpha1.OperationDeletedReason,
			outcome: opv1alpha1.CanceledCondition,
			others:  []condition.Cond{opv1alpha1.SucceededCondition, opv1alpha1.FailedCondition},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			initial := opv1alpha1.ETCDSnapshotRestoreStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase:        tc.phase,
					TerminatedAt: metav1.Now(),
				},
			}
			// What the phase handler recorded at decision time, which must survive to the end.
			tc.outcome.Reason(&initial, tc.reason)
			tc.outcome.Message(&initial, "the operative detail")

			got := updateStatus(newOp(), initial)

			assert.Equal(t, "True", tc.outcome.GetStatus(&got))
			assert.Equal(t, tc.reason, tc.outcome.GetReason(&got), "the decision-time reason must not be overwritten")
			assert.Equal(t, "the operative detail", tc.outcome.GetMessage(&got))

			for _, other := range tc.others {
				assert.Equal(t, "False", other.GetStatus(&got), "%s must be denied once another outcome is asserted", other)
			}

			assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&got))
			assert.Equal(t, opv1alpha1.FinishedReason, opv1alpha1.FinalizedCondition.GetReason(&got))
			assert.Equal(t, "False", opv1alpha1.PendingCondition.GetStatus(&got))
			assert.Equal(t, "False", opv1alpha1.InProgressCondition.GetStatus(&got))
		})
	}
}

// TestUpdateStatusFinalizedOnlyOnceTerminated pins the split between the two questions a waiter
// can ask. The outcome is asserted the moment the operation reaches its terminal phase, so
// `kubectl wait --for=condition=Succeeded` unblocks as soon as the work is done; Finalized is the
// one that waits for the controller to be finished with the operation, so pairing the two means
// "succeeded and fully wrapped up".
func TestUpdateStatusFinalizedOnlyOnceTerminated(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhasePending,
		opv1alpha1.OperationPhaseInProgress,
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseCanceled,
	} {
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			// Every phase, with the terminal marker deliberately absent — including the terminal
			// phases, which is the window a deletion would cancel.
			got := updateStatus(newOp(), opv1alpha1.ETCDSnapshotRestoreStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
			})

			assert.NotEqual(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&got),
				"Finalized must not be asserted before terminal handling completes")

			if !ops.IsTerminal(phase) {
				return
			}

			outcome, _ := outcomeConditionFor(phase)
			assert.Equal(t, "True", outcome.GetStatus(&got),
				"%s must be asserted as soon as the terminal phase is reached", outcome)
		})
	}
}
