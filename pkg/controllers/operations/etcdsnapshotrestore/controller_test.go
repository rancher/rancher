package etcdsnapshotrestore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1/snapshotutil"
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	rkecontrollers "github.com/rancher/rancher/pkg/generated/controllers/rke.cattle.io/v1"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/restoremode"
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
	dataDirErr        error
	provisioningDir   string
	kubectlPath       string
	kubectlPathErr    error
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
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(testClusterGVK)
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test-cluster")
	return cluster, nil
}

func (a *stubAdapter) BeaconRef() (string, string)    { return "test-namespace", "test-cluster" }
func (a *stubAdapter) WaitForRegister() (bool, error) { return a.waitForRegisterOK, nil }
func (a *stubAdapter) PauseCluster(_ bool) error      { return nil }
func (a *stubAdapter) RuntimeCommand() string         { return a.runtimeCommand }
func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) (string, error) {
	return a.dataDir, a.dataDirErr
}
func (a *stubAdapter) DistroManifestPaths(_ string) ops.ManifestPaths {
	return ops.ManifestPaths{}
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

func (a *stubAdapter) InstallInstruction(_ *corev1.Secret, dataDir string) (planapi.OneTimeInstruction, bool) {
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
				"RKE2_DATA_DIR=" + dataDir,
			},
		},
	}, true
}
func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string { return a.provisioningDir }
func (a *stubAdapter) ServerUnit() string                                { return a.serverUnit }
func (a *stubAdapter) RuntimeService(_ *corev1.Secret) string            { return a.serverUnit }
func (a *stubAdapter) DistroServices(secret *corev1.Secret) []string {
	return ops.DistroServices(a.runtimeCommand, secret)
}
func (a *stubAdapter) RenderProbes(_ *corev1.Secret, _ bool) (map[string]rkeplan.Probe, error) {
	return map[string]rkeplan.Probe{}, nil
}
func (a *stubAdapter) KubectlPath(_ *corev1.Secret) (string, error) {
	return a.kubectlPath, a.kubectlPathErr
}
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

// Update models the main-resource endpoint: Beacon has a status subresource, so a status change
// sent here is silently dropped. Keeping the fake honest about that is what stops a handler which
// clears beacon ownership through Update — as the stale-owner reclaim once did — from passing.
func (f *fakeBeaconClient) Update(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	updated := b.DeepCopy()
	if f.beacon != nil {
		updated.Status = f.beacon.Status
	}
	f.updates = append(f.updates, updated.DeepCopy())
	return updated, nil
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
var testOwnerKey = ops.BeaconOwnerKey(OperationKind, newOp())

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
// real controller computes so beacon fixtures created with testOwnerKey are recognized as ours.
func newScope(op *opv1alpha1.ETCDSnapshotRestore, beacon *planv1alpha1.Beacon) *scope {
	cluster, _ := defaultAdapter().ClusterObject()
	return &scope{
		ownerKey:   ops.BeaconOwnerKey(OperationKind, op),
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

	plan, skipReason, err := buildPostRestoreNodeCleanupPlan(s, initSecret, allSecrets)
	if err != nil {
		t.Fatal(err)
	}
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
	kubectlPath, err := s.adapter.KubectlPath(initSecret)
	if err != nil {
		t.Fatal(err)
	}
	if !envSet["KUBECTL="+kubectlPath] {
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
	plan, skipReason, err := buildPostRestoreNodeCleanupPlan(s, initSecret, []*corev1.Secret{initSecret})
	if err != nil {
		t.Fatal(err)
	}
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
	plan, skipReason, err := buildPostRestoreNodeCleanupPlan(s, initSecret, []*corev1.Secret{initSecret})
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Error("expected nil plan when kubectl path is missing")
	}
	if skipReason == "" {
		t.Error("expected non-empty skipReason when kubectl path is missing")
	}
}

func TestBuildPostRestoreNodeCleanupPlan_KubectlPathErrorPropagates(t *testing.T) {
	t.Parallel()

	a := defaultAdapter()
	a.kubectlPathErr = fmt.Errorf("kubectl path unavailable")
	s := newTestScope(a, "restore-uid")
	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel: "true",
		capr.InitNodeLabel: "true",
	})

	// An error resolving kubectl's path is a real failure to retry, not the same as the
	// adapter successfully reporting no kubectl/kubeconfig configured.
	plan, skipReason, err := buildPostRestoreNodeCleanupPlan(s, initSecret, []*corev1.Secret{initSecret})
	if err == nil {
		t.Fatal("expected error when adapter.KubectlPath fails")
	}
	if plan != nil {
		t.Error("expected nil plan when adapter.KubectlPath fails")
	}
	if skipReason != "" {
		t.Errorf("expected empty skipReason on error, got %q", skipReason)
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

	plan, err := buildPreflightPlan(s, secret)
	if err != nil {
		t.Fatal(err)
	}

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

func TestBuildPreflightPlan_DataDirectoryErrorPropagates(t *testing.T) {
	t.Parallel()

	a := defaultAdapter()
	a.dataDirErr = fmt.Errorf("data directory unavailable")
	s := newTestScope(a, "restore-uid")
	secret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel: "true",
		capr.InitNodeLabel: "true",
	})

	plan, err := buildPreflightPlan(s, secret)
	if err == nil {
		t.Fatal("expected error when adapter.DistroDataDirectory fails")
	}
	if plan != nil {
		t.Error("expected nil plan when adapter.DistroDataDirectory fails")
	}
}

func TestBuildShutdownPlan(t *testing.T) {
	t.Parallel()

	adapter := defaultAdapter()
	adapter.installVersion = "v1.33.0+rke2r1"
	s := newTestScope(adapter, "restore-uid")

	t.Run("etcd and control plane node", func(t *testing.T) {
		secret := makePlanSecret("init", "node-init", map[string]string{
			capr.EtcdRoleLabel:         "true",
			capr.ControlPlaneRoleLabel: "true",
		})

		plan, err := buildShutdownPlan(s, secret)
		if err != nil {
			t.Fatal(err)
		}

		var names []string
		for _, instr := range plan.OneTimeInstructions {
			names = append(names, instr.Name)
		}
		// The install is ordered ahead of the killall, so anything it starts is torn down again.
		want := []string{"remove idempotency tracking", "shutdown", "create-etcd-tombstone", "remove-tls-directory"}
		if strings.Join(names, ",") != strings.Join(want, ",") {
			t.Errorf("instructions = %v, want %v", names, want)
		}

		// The killall script reads the data directory out of the environment.
		// The install now sits between the tracking cleanup and the killall, so the shutdown is third.
		shutdown := plan.OneTimeInstructions[1]
		dataDir, err := adapter.DistroDataDirectory(secret)
		if err != nil {
			t.Fatal(err)
		}
		wantEnv := fmt.Sprintf("%s_DATA_DIR=%s", strings.ToUpper(adapter.RuntimeCommand()), dataDir)
		if !slices.Contains(shutdown.Env, wantEnv) {
			t.Errorf("shutdown instruction env = %v, want it to contain %q", shutdown.Env, wantEnv)
		}

		if len(plan.Files) != 1 || plan.Files[0].Path != ops.IdempotentActionScriptPath(adapter.ProvisioningDataDirectory(secret)) {
			t.Errorf("expected the idempotent script file, got %v", plan.Files)
		}
	})

	t.Run("worker node", func(t *testing.T) {
		secret := makePlanSecret("worker-1", "node-worker-1", map[string]string{
			capr.WorkerRoleLabel: "true",
		})

		plan, err := buildShutdownPlan(s, secret)
		if err != nil {
			t.Fatal(err)
		}

		// No etcd data or TLS material to clear on a worker, but it is still installed and stopped:
		// it has to come back on the same version as the control plane it rejoins.
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

func TestBuildShutdownPlan_DataDirectoryErrorPropagates(t *testing.T) {
	t.Parallel()

	a := defaultAdapter()
	a.dataDirErr = fmt.Errorf("data directory unavailable")
	s := newTestScope(a, "restore-uid")
	secret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel:         "true",
		capr.ControlPlaneRoleLabel: "true",
	})

	plan, err := buildShutdownPlan(s, secret)
	if err == nil {
		t.Fatal("expected error when adapter.DistroDataDirectory fails")
	}
	if plan != nil {
		t.Error("expected nil plan when adapter.DistroDataDirectory fails")
	}
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
		build func(*scope) (*planapi.Plan, error)
		step  opv1alpha1.ETCDSnapshotRestoreStep
	}{
		"preflight": {
			build: func(s *scope) (*planapi.Plan, error) { return buildPreflightPlan(s, secret) },
			step:  opv1alpha1.ETCDSnapshotRestoreStepPreflight,
		},
		"shutdown": {
			build: func(s *scope) (*planapi.Plan, error) { return buildShutdownPlan(s, secret) },
			step:  opv1alpha1.ETCDSnapshotRestoreStepShutdown,
		},
	}

	for name, b := range builders {
		t.Run(name, func(t *testing.T) {
			marshal := func(uid types.UID) string {
				s := newTestScope(defaultAdapter(), uid)
				p, err := b.build(s)
				if err != nil {
					t.Fatal(err)
				}
				p = ops.WithOperationEnv(p, ops.OperationEnv(ControllerOwnerKey, s.op, b.step))
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

// provClusterWithDrainTimeout renders a v2prov cluster whose control-plane drain timeout is the
// given value. It goes through ToUnstructured because both sides of a restore do: CAPRAdapter
// renders the restore target that way, and snapshotextrametadata publishes the captured cluster
// that way too (see its sanitize). So the captured and live subtrees have the same shape, and the
// Go types their numbers carry are the only thing that can disagree.
func provClusterWithDrainTimeout(t *testing.T, timeout int) *unstructured.Unstructured {
	t.Helper()

	cluster := &provv1.Cluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: "provisioning.cattle.io/v1", Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "fleet-default"},
		Spec: provv1.ClusterSpec{
			KubernetesVersion: "v1.34.1+rke2r1",
			RKEConfig: &provv1.RKEConfig{
				ClusterConfiguration: rkev1.ClusterConfiguration{
					UpgradeStrategy: rkev1.ClusterUpgradeStrategy{
						ControlPlaneDrainOptions: rkev1.DrainOptions{Enabled: true, Timeout: timeout},
					},
				},
			},
		},
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cluster)
	if err != nil {
		t.Fatalf("rendering the cluster: %v", err)
	}

	return &unstructured.Unstructured{Object: obj}
}

// roundTripProvCluster models what happens to a restore target between two reconciles: CAPRAdapter
// decodes the object it was handed into a provv1.Cluster to update it, and the next RestoreTarget
// call renders the stored object back with ToUnstructured. That round trip is what settles a field
// on the type the unstructured convention gives it, so a test asserting convergence has to go
// through it rather than reusing the in-memory copy the write was built from. It also fails loudly
// on a value the typed object cannot hold.
func roundTripProvCluster(t *testing.T, obj *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()

	cluster := &provv1.Cluster{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, cluster); err != nil {
		t.Fatalf("decoding the updated cluster: %v", err)
	}

	stored, err := runtime.DefaultUnstructuredConverter.ToUnstructured(cluster)
	if err != nil {
		t.Fatalf("rendering the stored cluster: %v", err)
	}

	return &unstructured.Unstructured{Object: stored}
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

	// A writable path is matched whole, so a subtree like upgradeStrategy is restored as one value
	// and every number inside it takes part in the comparison. Those only converge if the captured
	// numbers and the live ones agree on their Go type, which is why this case goes through
	// resolveRestoreMode rather than a hand-built Match: the captured value has to come out of a
	// real snapshot payload, which is where the type is decided. A payload decoded as float64 still
	// writes onto the target, so nothing looks wrong until the second pass keeps reporting
	// "applied" and the restore-mode step never advances.
	t.Run("a numeric field converges on the pass after it is written", func(t *testing.T) {
		upgradeStrategyPath := []string{"spec", "rkeConfig", "upgradeStrategy"}
		timeoutPath := append(append([]string{}, upgradeStrategyPath...), "controlPlaneDrainOptions", "timeout")

		snapshot := snapshotWithModes(t, map[string]string{
			rkev1.RestoreRKEConfigAll: restoremode.Match{ResourceKey: rkev1.SnapshotResourceProvCluster, Path: upgradeStrategyPath}.String(),
		}, map[string]any{
			rkev1.SnapshotResourceProvCluster: provClusterWithDrainTimeout(t, 30).Object,
		}, "none,"+rkev1.RestoreRKEConfigAll)

		h := &handler{etcdsnapshots: &stubSnapshotClient{snapshot: snapshot}}
		matches, reason, err := h.resolveRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, defaultAdapter()), rkev1.RestoreRKEConfigAll)
		if err != nil || reason != "" {
			t.Fatalf("err = %v, reason = %s", err, reason)
		}
		if len(matches) != 1 {
			t.Fatalf("expected 1 match, got %d: %v", len(matches), matches)
		}

		// The cluster currently drains with a different timeout.
		target := provClusterWithDrainTimeout(t, 60)

		adapter := defaultAdapter()
		adapter.restoreTargets = map[string]*unstructured.Unstructured{rkev1.SnapshotResourceProvCluster: target}

		applied, reason, err := h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, adapter), matches)
		if err != nil || reason != "" {
			t.Fatalf("first pass: err = %v, reason = %s", err, reason)
		}
		if !applied {
			t.Fatal("first pass: expected the captured timeout to be applied")
		}
		if len(adapter.updatedTargets) != 1 {
			t.Fatalf("first pass: expected 1 update, got %d", len(adapter.updatedTargets))
		}

		// The next reconcile reads the object back after it was persisted, not the in-memory copy
		// the write was built from.
		persisted := roundTripProvCluster(t, adapter.updatedTargets[0])
		got, found, err := unstructured.NestedInt64(persisted.Object, timeoutPath...)
		if err != nil || !found {
			t.Fatalf("timeout not persisted as an integer: found=%v err=%v", found, err)
		}
		if got != 30 {
			t.Errorf("timeout = %d, want the captured value", got)
		}

		next := defaultAdapter()
		next.restoreTargets = map[string]*unstructured.Unstructured{rkev1.SnapshotResourceProvCluster: persisted}

		applied, reason, err = h.applyRestoreMode(resolveScope(rkev1.RestoreRKEConfigAll, next), matches)
		if err != nil || reason != "" {
			t.Fatalf("second pass: err = %v, reason = %s", err, reason)
		}
		if applied {
			t.Error("second pass: expected the step to converge once the timeout already holds the captured value")
		}
		if len(next.updatedTargets) != 0 {
			t.Errorf("second pass: expected 0 updates, got %d", len(next.updatedTargets))
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

	t.Run("fails when the mode cannot be resolved", func(t *testing.T) {
		adapter := defaultAdapter()
		h := &handler{etcdsnapshots: &stubSnapshotClient{notFound: true}}
		s := resolveScope(rkev1.RestoreRKEConfigAll, adapter)

		status, err := h.reconcileRestoreClusterConfig(s, opv1alpha1.ETCDSnapshotRestoreStatus{
			Step: opv1alpha1.ETCDSnapshotRestoreStepRestoreClusterConfig,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if status.Phase != opv1alpha1.OperationPhaseFailed {
			t.Errorf("phase = %q, want Failed", status.Phase)
		}
	})

	t.Run("failed rather than silently restoring nothing", func(t *testing.T) {
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
		if status.Phase != opv1alpha1.OperationPhaseFailed {
			t.Errorf("phase = %q, want Failed", status.Phase)
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
func TestBuildRestorePlanInstructions(t *testing.T) {
	t.Parallel()

	secret := makePlanSecret("etcd-0", "node-etcd-0", map[string]string{capr.EtcdRoleLabel: "true"})

	t.Run("the plan wipes etcd and resets, and installs nothing", func(t *testing.T) {
		t.Parallel()

		// The adapter does offer an install, so this pins that the restore deliberately leaves it to
		// the Shutdown step rather than merely having nothing to install: by the time the reset runs,
		// the snapshot's own binary is already on disk.
		adapter := defaultAdapter()
		adapter.installVersion = "v1.33.0+rke2r1"
		s := newTestScope(adapter, types.UID("uid-1"))

		nodePlan, err := buildRestorePlan(s, secret, nil, "snapshot-1")
		if err != nil {
			t.Fatalf("buildRestorePlan: %v", err)
		}

		assertInstructionOrder(t, nodePlan, []string{idempotencyKey + "/clean-etcd-dir", idempotencyKey + "/restore"})
	})

	t.Run("the instructions are scoped to this operation", func(t *testing.T) {
		t.Parallel()

		// Every restore instruction runs through the idempotency wrapper keyed on the op's UID, so a
		// re-reconcile does not reset etcd twice and two operations never share tracking state.
		adapter := defaultAdapter()

		first, err := buildRestorePlan(newTestScope(adapter, types.UID("uid-1")), secret, nil, "snapshot-1")
		if err != nil {
			t.Fatalf("buildRestorePlan: %v", err)
		}
		second, err := buildRestorePlan(newTestScope(adapter, types.UID("uid-2")), secret, nil, "snapshot-1")
		if err != nil {
			t.Fatalf("buildRestorePlan: %v", err)
		}

		if fmt.Sprint(first.OneTimeInstructions[0].Args) == fmt.Sprint(second.OneTimeInstructions[0].Args) {
			t.Error("expected the restore instructions to be scoped to the operation UID")
		}
		if !slices.Contains(first.OneTimeInstructions[0].Args, idempotencyKey+"/clean-etcd-dir") {
			t.Errorf("args = %v, want the clean-etcd-dir idempotency key", first.OneTimeInstructions[0].Args)
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

func TestBuildRestartPlan(t *testing.T) {
	t.Parallel()

	const (
		serverURL          = "10.0.0.1"
		initialValue       = "uid-1/initial"
		finalValue         = "uid-1/final"
		restartTestVersion = "v1.33.0+rke2r1"
	)

	initSecret := makePlanSecret("init", "node-init", map[string]string{
		capr.EtcdRoleLabel:         "true",
		capr.ControlPlaneRoleLabel: "true",
		capr.InitNodeLabel:         "true",
	})
	otherEtcd := makePlanSecret("etcd-2", "node-etcd-2", map[string]string{
		capr.EtcdRoleLabel: "true",
	})
	worker := makePlanSecret("worker", "node-worker", map[string]string{
		capr.WorkerRoleLabel: "true",
	})

	t.Run("each pass restarts without reinstalling", func(t *testing.T) {
		t.Parallel()

		// The version was installed during the Shutdown step, on every node at once, so a restart is
		// all that is left to do. The adapter still offers an install, so this pins that the restart
		// leaves it alone rather than there being nothing to install.
		for _, secret := range []*corev1.Secret{initSecret, otherEtcd, worker} {
			adapter := defaultAdapter()
			adapter.installVersion = restartTestVersion
			s := newTestScope(adapter, types.UID("uid-1"))

			initial, err := buildRestartPlan(s, secret, initSecret, serverURL, initialValue, true)
			if err != nil {
				t.Fatalf("node %s: buildRestartPlan: %v", secret.Name, err)
			}
			assertInstructionOrder(t, initial, []string{idempotencyKey + "/restart"})

			final, err := buildRestartPlan(s, secret, initSecret, serverURL, finalValue, false)
			if err != nil {
				t.Fatalf("node %s: buildRestartPlan: %v", secret.Name, err)
			}
			assertInstructionOrder(t, final, []string{idempotencyKey + "/restart", "remove-server-arg"})
		}
	})

	t.Run("worker nodes restart the agent unit", func(t *testing.T) {
		t.Parallel()

		adapter := defaultAdapter()
		adapter.installVersion = restartTestVersion
		s := newTestScope(adapter, types.UID("uid-1"))

		for _, tc := range []struct {
			secret *corev1.Secret
			want   string
		}{
			{secret: initSecret, want: "rke2-server"},
			{secret: otherEtcd, want: "rke2-server"},
			{secret: worker, want: "rke2-agent"},
		} {
			nodePlan, err := buildRestartPlan(s, tc.secret, initSecret, serverURL, initialValue, true)
			if err != nil {
				t.Fatalf("node %s: buildRestartPlan: %v", tc.secret.Name, err)
			}
			if args := restartInstructionArgs(t, nodePlan); !slices.Contains(args, tc.want) {
				t.Errorf("node %s: restart args = %v, want the %s unit", tc.secret.Name, args, tc.want)
			}
		}
	})

	t.Run("the server drop-in points the non-leader nodes at the restored node", func(t *testing.T) {
		t.Parallel()

		adapter := defaultAdapter()
		adapter.installVersion = restartTestVersion
		s := newTestScope(adapter, types.UID("uid-1"))
		dropIn := path.Join(adapter.ConfigDirectory(otherEtcd), "zz_etcd-snapshot-restore.yaml")

		// Initial pass: every node but the leader gets the drop-in, and nothing removes it yet.
		nodePlan, err := buildRestartPlan(s, otherEtcd, initSecret, serverURL, initialValue, true)
		if err != nil {
			t.Fatalf("buildRestartPlan: %v", err)
		}
		if len(nodePlan.Files) != 2 {
			t.Fatalf("files = %d, want the idempotency script plus the drop-in", len(nodePlan.Files))
		}
		if nodePlan.Files[1].Path != dropIn {
			t.Errorf("drop-in path = %q, want %q", nodePlan.Files[1].Path, dropIn)
		}
		content, err := base64.StdEncoding.DecodeString(nodePlan.Files[1].Content)
		if err != nil {
			t.Fatalf("decoding drop-in: %v", err)
		}
		if want := "server: \"https://10.0.0.1:9345\"\n"; string(content) != want {
			t.Errorf("drop-in = %q, want %q", string(content), want)
		}

		// The leader is the node being pointed at, so it must not be told to join itself.
		leaderPlan, err := buildRestartPlan(s, initSecret, initSecret, serverURL, initialValue, true)
		if err != nil {
			t.Fatalf("buildRestartPlan: %v", err)
		}
		if len(leaderPlan.Files) != 1 {
			t.Errorf("leader files = %d, want no server drop-in", len(leaderPlan.Files))
		}

		// Final pass: the drop-in is removed everywhere, the leader included.
		for _, secret := range []*corev1.Secret{initSecret, otherEtcd, worker} {
			finalPlan, err := buildRestartPlan(s, secret, initSecret, serverURL, finalValue, false)
			if err != nil {
				t.Fatalf("node %s: buildRestartPlan: %v", secret.Name, err)
			}
			if len(finalPlan.Files) != 1 {
				t.Errorf("node %s: files = %d, want the final pass to write no drop-in", secret.Name, len(finalPlan.Files))
			}
			last := finalPlan.OneTimeInstructions[len(finalPlan.OneTimeInstructions)-1]
			if last.Name != "remove-server-arg" {
				t.Errorf("node %s: last instruction = %q, want remove-server-arg", secret.Name, last.Name)
			}
			if !slices.Contains(last.Args, dropIn) {
				t.Errorf("node %s: remove args = %v, want %q", secret.Name, last.Args, dropIn)
			}
		}
	})
}

// restartInstructionArgs returns the arguments of the plan's restart instruction. It is wrapped by
// the idempotency script, so the real command and its arguments sit at the tail of Args.
func restartInstructionArgs(t *testing.T, nodePlan *planapi.Plan) []string {
	t.Helper()

	for _, inst := range nodePlan.OneTimeInstructions {
		if instructionIdentifier(inst) == idempotencyKey+"/restart" {
			return inst.Args
		}
	}
	t.Fatalf("plan has no restart instruction: %v", nodePlan.OneTimeInstructions)
	return nil
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
	"aborted": {
		handle: (*handler).handleAborted,
		cond:   opv1alpha1.AbortedCondition,
		hook:   opv1alpha1.AbortedPhaseHookLabelPrefix,
	},
	"canceled": {
		handle: (*handler).handleCanceled,
		cond:   opv1alpha1.CanceledCondition,
		hook:   opv1alpha1.CanceledPhaseHookLabelPrefix,
	},
	"failed": {
		handle: (*handler).handleFailed,
		cond:   opv1alpha1.FailedCondition,
		hook:   opv1alpha1.FailedPhaseHookLabelPrefix,
	},
	"succeeded": {
		handle: (*handler).handleSucceeded,
		cond:   opv1alpha1.SucceededCondition,
		hook:   opv1alpha1.SucceededPhaseHookLabelPrefix,
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

// TestHandleTerminal_WithoutBeaconClaimLeavesItUntouched covers every outcome an operation can
// reach without holding the beacon — Failed after losing it, Aborted after being overtaken,
// Canceled by whoever wanted it next. In all three the operation still finishes, and the beacon
// (now someone else's) is left exactly as it is: not cleared, and not carrying the phase hook's
// delegate, which is the write that would otherwise reach into another controller's operation.
// Succeeded is excluded: it cannot be reached without holding the beacon throughout.
func TestHandleTerminal_WithoutBeaconClaimLeavesItUntouched(t *testing.T) {
	t.Parallel()

	for name, tc := range terminalHandlers {
		if name == "succeeded" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			op := newOp()
			op.Labels = map[string]string{tc.hook + "cleanup": "delegate-a"}

			beacons := &fakeBeaconClient{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(op, newBeacon("another-controller", true))

			got, err := tc.handle(h, s, opv1alpha1.ETCDSnapshotRestoreStatus{})
			assert.NoError(t, err)
			assert.False(t, got.TerminatedAt.IsZero(),
				"with no beacon to release, terminal handling is trivially complete")
			assert.Empty(t, beacons.statusUpdates, "a beacon held by another controller must not be modified")
			assert.Empty(t, beacons.updates)
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
	op.Labels = map[string]string{opv1alpha1.CanceledPhaseHookLabelPrefix + "test": "delegate-a"}
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

// TestOnChange_DeletionOfPausedOperation covers a paused operation being deleted. Pausing stops the
// controller touching the operation at all, and tearing it down is no exception: its beacon is left
// alone and its finalizer stays, so the deletion waits for the pause to lift. Resuming the
// operation is what lets it finish deleting.
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
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, status.Phase, "a paused operation is not reconciled, even to cancel it")
	assert.Equal(t, "True", opv1alpha1.PausedCondition.GetStatus(&status), "its conditions are still refreshed")
	assert.Empty(t, beacons.statusUpdates, "the beacon must be left exactly as the pause found it")
	assert.Empty(t, controller.updates, "the finalizer must stay until the operation is resumed")
	assert.Zero(t, controller.enqueueCalls, "a paused operation is not polled")

	// Resumed, the deletion proceeds as it would have in the first place.
	op.Spec.Paused = false
	op.Status = status

	status, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.NotEmpty(t, beacons.statusUpdates, "the beacon is released once the operation is resumed")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1, "the finalizer goes once terminal handling completes") {
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

// TestOnChange_PausedOperationDoesNotTakeFinalizer follows from a paused operation not being
// reconciled at all. It matters most for one which was paused before it ever ran: having dispatched
// nothing, it has nothing to tear down, and a finalizer would only stand between the user and
// deleting it.
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
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		Phase:       opv1alpha1.OperationPhaseSucceeded,
		LastUpdated: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		Step:        opv1alpha1.ETCDSnapshotRestoreStepRestartCluster,
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
			others:  []condition.Cond{opv1alpha1.FailedCondition, opv1alpha1.AbortedCondition, opv1alpha1.CanceledCondition},
		},
		{
			name:    "failed",
			phase:   opv1alpha1.OperationPhaseFailed,
			reason:  opv1alpha1.PlanFailedReason,
			outcome: opv1alpha1.FailedCondition,
			others:  []condition.Cond{opv1alpha1.SucceededCondition, opv1alpha1.AbortedCondition, opv1alpha1.CanceledCondition},
		},
		{
			name:    "canceled",
			phase:   opv1alpha1.OperationPhaseCanceled,
			reason:  opv1alpha1.OperationDeletedReason,
			outcome: opv1alpha1.CanceledCondition,
			others:  []condition.Cond{opv1alpha1.SucceededCondition, opv1alpha1.FailedCondition, opv1alpha1.AbortedCondition},
		},
		{
			name:    "aborted",
			phase:   opv1alpha1.OperationPhaseAborted,
			reason:  opv1alpha1.PreflightCheckFailedReason,
			outcome: opv1alpha1.AbortedCondition,
			others:  []condition.Cond{opv1alpha1.SucceededCondition, opv1alpha1.FailedCondition, opv1alpha1.CanceledCondition},
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
		opv1alpha1.OperationPhaseAborted,
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

			outcome, _ := opv1alpha1.OutcomeConditionFor(phase)
			assert.Equal(t, "True", outcome.GetStatus(&got),
				"%s must be asserted as soon as the terminal phase is reached", outcome)
		})
	}
}

// --- cancellation ----------------------------------------------------------------------------

// TestOnChange_CancelRequestedCancelsInFlightOperation covers the core of the cancellation
// contract, which mirrors the deletion one: an operation canceled while it is still running stops
// where it is, releases the beacon, and reports why it was canceled.
func TestOnChange_CancelRequestedCancelsInFlightOperation(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Cancel = true
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepShutdown,
	}

	h, _, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.Equal(t, "True", opv1alpha1.CanceledCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.False(t, status.TerminatedAt.IsZero(), "handleCanceled ran to completion, so it must be recorded")
	assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&status))
	if assert.Len(t, beacons.statusUpdates, 1, "the beacon must be released") {
		assert.Equal(t, "", beacons.statusUpdates[0].Status.Owner)
		assert.False(t, beacons.statusUpdates[0].Status.Active)
	}
}

// TestOnChange_CancelRequestedInTerminalPhaseIsDeclined covers the edge of the window: the
// operation's work is over and its outcome asserted, so there is nothing for a cancellation to
// stop, even though its terminal phase hook is still holding the beacon. Deleting the operation is
// what breaks that deadlock.
func TestOnChange_CancelRequestedInTerminalPhaseIsDeclined(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "wedged": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup,
	}

	beacon := newBeacon(testOwnerKey, true)
	beacon.Status.Delegates = []string{"delegate-a"}
	h, _, beacons := newOnChangeHandler(beacon)

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase,
		"the operation had already concluded, so the phase it ended in stands")
	assert.Equal(t, "True", opv1alpha1.SucceededCondition.GetStatus(&status), "its outcome must survive the request")
	assert.Equal(t, "False", opv1alpha1.CanceledCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.CancellationDeclinedReason, opv1alpha1.CanceledCondition.GetReason(&status),
		"a declined cancellation must be acknowledged, not silently passed over")
	assert.True(t, status.TerminatedAt.IsZero(), "the hook is still owed an answer, so handling is not complete")
	for _, update := range beacons.statusUpdates {
		assert.Equal(t, testOwnerKey, update.Status.Owner, "the beacon must stay with the operation and its delegate")
	}
}

// TestOnChange_CancelRequestedAfterTerminationKeepsOutcome is the same rule for an operation the
// controller has fully finished with: nothing left to call off, and by then nothing left to release
// either.
func TestOnChange_CancelRequestedAfterTerminationKeepsOutcome(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Status = updateStatus(op, opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:        opv1alpha1.OperationPhaseSucceeded,
			TerminatedAt: metav1.Now(),
		},
		Step: opv1alpha1.ETCDSnapshotRestoreStepPostRestorePodCleanup,
	})

	h, _, beacons := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase, "a terminated operation must not be demoted to Canceled")
	assert.Equal(t, "True", opv1alpha1.SucceededCondition.GetStatus(&status))
	assert.Equal(t, "False", opv1alpha1.CanceledCondition.GetStatus(&status), "the Canceled condition must be denied, not raised")
	assert.Equal(t, opv1alpha1.CancellationDeclinedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.Empty(t, beacons.statusUpdates, "there is nothing left to release")
}

// TestOnChange_PausedBlocksCancel pins the documented precedence: Paused halts reconciliation
// outright, so a cancellation requested on a paused operation is not acted on until it is resumed.
func TestOnChange_PausedBlocksCancel(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true
	op.Spec.Cancel = true
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepShutdown,
	}

	h, controller, beacons := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, status.Phase, "a paused operation must not transition on cancel")
	assert.Equal(t, "", opv1alpha1.CanceledCondition.GetStatus(&status), "the Canceled condition must not be reported while paused")
	assert.Equal(t, "True", opv1alpha1.PausedCondition.GetStatus(&status))
	assert.Empty(t, beacons.statusUpdates, "the beacon must be left as the pause found it")
	assert.Empty(t, controller.updates, "a paused operation is not finalized, so it takes no finalizer")
}

// TestUpdateStatusReportsDeclinedCancellation covers the acknowledgement on its own, across every
// outcome an operation can end in.
func TestUpdateStatusReportsDeclinedCancellation(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
	} {
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			initial := opv1alpha1.ETCDSnapshotRestoreStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
			}

			op := newOp()
			got := updateStatus(op, initial)
			assert.Equal(t, opv1alpha1.NotCanceledReason, opv1alpha1.CanceledCondition.GetReason(&got),
				"with no request to report, the denial stays generic")

			op.Spec.Cancel = true
			got = updateStatus(op, initial)
			assert.Equal(t, "False", opv1alpha1.CanceledCondition.GetStatus(&got),
				"the operation was not canceled, so the condition stays denied")
			assert.Equal(t, opv1alpha1.CancellationDeclinedReason, opv1alpha1.CanceledCondition.GetReason(&got))
			assert.Contains(t, opv1alpha1.CanceledCondition.GetMessage(&got), string(phase),
				"the message should say which phase the operation had already reached")

			outcome, _ := opv1alpha1.OutcomeConditionFor(phase)
			assert.Equal(t, "True", outcome.GetStatus(&got), "the request must not disturb the outcome")
		})
	}

	// An operation which really was canceled keeps the reason it was canceled for: the
	// acknowledgement is for requests that were *not* acted on.
	t.Run("canceled keeps its own reason", func(t *testing.T) {
		t.Parallel()

		op := newOp()
		op.Spec.Cancel = true

		status := opv1alpha1.ETCDSnapshotRestoreStatus{
			OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseCanceled},
		}
		status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")

		got := updateStatus(op, status)
		assert.Equal(t, "True", opv1alpha1.CanceledCondition.GetStatus(&got))
		assert.Equal(t, opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(&got))
	})
}

// TestHandlePending_ReclaimsSupersededClaim covers the wiring of the no-lookup reclaim. A beacon
// still carrying a claim from an earlier object of this operation's name would otherwise leave the
// operation waiting on a holder that no longer exists, and the delegate that claim left behind —
// pushed to gate a hook that died with the object, so nothing will ever pop it — would be handed to
// the operation that acquires next.
func TestHandlePending_ReclaimsSupersededClaim(t *testing.T) {
	t.Parallel()

	op := newOp()
	superseded := ops.BeaconOwnerKey(OperationKind, &opv1alpha1.ETCDSnapshotRestore{
		ObjectMeta: metav1.ObjectMeta{Namespace: op.Namespace, Name: op.Name, UID: "dead-uid"},
	})

	beacon := newBeacon(superseded, true)
	beacon.Status.Delegates = []string{"dead-delegate"}

	beacons := &fakeBeaconClient{beacon: beacon}
	h := &handler{beacons: beacons}
	s := newScope(op, beacon)

	_, err := h.handlePending(s, opv1alpha1.ETCDSnapshotRestoreStatus{})
	assert.NoError(t, err)
	assert.Equal(t, testOwnerKey, s.beacon.Status.Owner, "the operation must end up holding the beacon")
	assert.Empty(t, s.beacon.Status.Delegates, "the dead claim's delegate must not be inherited")
}

// missingClusterOp points an operation at a cluster the dynamic resolver does not serve, which is
// what the reconcile sees once a cluster has been deleted underneath an operation.
func missingClusterOp(op *opv1alpha1.ETCDSnapshotRestore) *opv1alpha1.ETCDSnapshotRestore {
	op.Spec.ClusterRef.Name = "gone"
	return op
}

// --- a missing cluster or beacon for an operation which has already concluded ------------------

// TestOnChange_CancelWithMissingClusterKeepsOutcome covers the sequence that motivated this rule:
// cancellation is applied before the scope is resolved, so a canceled operation whose cluster is
// gone reaches the missing-cluster branch already terminal. Failing it there would report the
// user's cancellation as a failure.
func TestOnChange_CancelWithMissingClusterKeepsOutcome(t *testing.T) {
	t.Parallel()

	op := missingClusterOp(newOp())
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	h, _, _ := newOnChangeHandler(newBeacon(testOwnerKey, true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase, "the cancellation must not be reported as a failure")
	assert.Equal(t, opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.Equal(t, "False", opv1alpha1.FailedCondition.GetStatus(&status))
	assert.NotEqual(t, opv1alpha1.ClusterNotFoundReason, opv1alpha1.FailedCondition.GetReason(&status))
	assert.False(t, status.TerminatedAt.IsZero(), "with no cluster there is nothing to release")
	assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&status))
}

// The same rule protects an operation which concluded on an earlier reconcile: deleting the cluster
// inside the operation's TTL must not rewrite what it ended with.
func TestOnChange_MissingClusterKeepsConcludedOutcome(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
		opv1alpha1.OperationPhaseCanceled,
	} {
		t.Run(string(phase), func(t *testing.T) {
			op := missingClusterOp(newOp())
			op.Spec.TTL = -1

			initial := opv1alpha1.ETCDSnapshotRestoreStatus{Step: opv1alpha1.ETCDSnapshotRestoreStepRestore}
			initial.SetPhase(phase)
			op.Status = initial

			h, _, _ := newOnChangeHandler(newBeacon("", false))

			status, err := h.OnChange(op, op.Status)
			assert.NoError(t, err)
			assert.Equal(t, phase, status.Phase, "the phase the operation ended in must stand")

			outcome, _ := opv1alpha1.OutcomeConditionFor(phase)
			assert.Equal(t, "True", outcome.GetStatus(&status))
			assert.False(t, status.TerminatedAt.IsZero(), "with no cluster there is nothing to release")
		})
	}
}

// TestOnChange_MissingClusterWithOwedHookDefersTermination is the other half of the rule. The
// operation cannot be handed a beacon that is not there, but its terminal phase hook is still
// labelled: the delegate has not had its turn, so recording termination would claim it had.
func TestOnChange_MissingClusterWithOwedHookDefersTermination(t *testing.T) {
	t.Parallel()

	op := missingClusterOp(newOp())
	op.Spec.TTL = -1
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}

	initial := opv1alpha1.ETCDSnapshotRestoreStatus{Step: opv1alpha1.ETCDSnapshotRestoreStepRestore}
	initial.MarkSucceeded()
	op.Status = initial

	h, _, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase)
	assert.True(t, status.TerminatedAt.IsZero(), "the delegate has not had its turn, so nothing may be recorded")
	assert.Equal(t, "False", opv1alpha1.FinalizedCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.FinalizedCondition.GetReason(&status),
		"the wait must be reported against the delegate holding it up")
}

// A deleting operation is the deliberate exception: it is being discarded and its hooks go with it,
// so it terminates at once rather than holding its finalizer open for a delegate that will never be
// handed anything.
func TestOnChange_DeletionWithMissingClusterAbandonsOwedHook(t *testing.T) {
	t.Parallel()

	op := missingClusterOp(newDeletingOp())
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
	}

	h, controller, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.False(t, status.TerminatedAt.IsZero(), "a deletion abandons the hook rather than waiting on it")

	op.Status = status

	_, err = h.OnChange(op, op.Status)
	assert.NoError(t, err)
	if assert.Len(t, controller.updates, 1, "the finalizer must not be held for an abandoned hook") {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestOnChange_MissingBeaconDisposition covers what an operation does when the beacon it needs is
// not there. What it should do depends entirely on what it still owes: an outcome which dispatched
// work owes a release and complains that it cannot make one, while an outcome which dispatched
// nothing owes nothing and finishes.
func TestOnChange_MissingBeaconDisposition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// phase the operation is in when its beacon turns up missing.
		phase opv1alpha1.OperationPhase
		// terminated is whether it had already recorded terminal handling as complete.
		terminated bool

		wantErr        bool
		wantPhase      opv1alpha1.OperationPhase
		wantReason     string
		wantTerminated bool
	}{
		{
			// Aborted called its own work off, so there is nothing a beacon would have wound down.
			name:           "aborted finishes",
			phase:          opv1alpha1.OperationPhaseAborted,
			wantPhase:      opv1alpha1.OperationPhaseAborted,
			wantTerminated: true,
		},
		{
			// Cancellation is usually driven by whoever wants the beacon next, so a beacon that is
			// not there is no surprise at all.
			name:           "canceled finishes",
			phase:          opv1alpha1.OperationPhaseCanceled,
			wantPhase:      opv1alpha1.OperationPhaseCanceled,
			wantTerminated: true,
		},
		{
			// Work was dispatched under the beacon's authority and never handed back: the state
			// serializing writes to this cluster went missing while this operation still had a claim
			// on it. It stays stuck, and says so, rather than recording itself as wrapped up.
			name:      "succeeded complains and sticks",
			phase:     opv1alpha1.OperationPhaseSucceeded,
			wantErr:   true,
			wantPhase: opv1alpha1.OperationPhaseSucceeded,
		},
		{
			name:      "failed complains and sticks",
			phase:     opv1alpha1.OperationPhaseFailed,
			wantErr:   true,
			wantPhase: opv1alpha1.OperationPhaseFailed,
		},
		{
			// Already released whatever it held, so a beacon collected afterwards is none of its
			// business, and it must still be collectable.
			name:           "succeeded and already terminated settles",
			phase:          opv1alpha1.OperationPhaseSucceeded,
			terminated:     true,
			wantPhase:      opv1alpha1.OperationPhaseSucceeded,
			wantTerminated: true,
		},
		{
			// Still in flight: the beacon was taken out from under it, which is the same fault
			// handleInProgress reports when it finds the beacon reassigned.
			name:           "in progress fails",
			phase:          opv1alpha1.OperationPhaseInProgress,
			wantPhase:      opv1alpha1.OperationPhaseFailed,
			wantReason:     opv1alpha1.BeaconLostReason,
			wantTerminated: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			op := newOp()
			op.Spec.TTL = -1

			initial := opv1alpha1.ETCDSnapshotRestoreStatus{Step: opv1alpha1.ETCDSnapshotRestoreStepRestore}
			initial.SetPhase(tc.phase)
			if tc.terminated {
				initial.SetTerminated()
			}
			op.Status = initial

			h, _, _ := newOnChangeHandler(nil)

			status, err := h.OnChange(op, op.Status)
			if tc.wantErr {
				assert.Error(t, err, "the operation must complain about a beacon it cannot release")
			} else {
				assert.NoError(t, err)
			}

			// A handler which returns an error has its status reverted by the generated status
			// handler, so only the phase it was already in is observable on that path.
			assert.Equal(t, tc.wantPhase, status.Phase)
			if tc.wantReason != "" {
				outcome, _ := opv1alpha1.OutcomeConditionFor(status.Phase)
				assert.Equal(t, tc.wantReason, outcome.GetReason(&status))
			}
			assert.Equal(t, tc.wantTerminated, !status.TerminatedAt.IsZero(),
				"termination is recorded only when nothing is owed")
		})
	}
}

// A Pending operation has not acquired the beacon yet, so its absence is "not created" rather than
// "lost" — the system-agent controller creates one once the cluster can take operations — and the
// operation waits rather than failing.
func TestOnChange_MissingBeaconWhilePendingWaits(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.TTL = -1
	op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhasePending},
	}

	h, _, _ := newOnChangeHandler(nil)

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhasePending, status.Phase, "a beacon yet to be created is not a failure")
	assert.Equal(t, opv1alpha1.WaitingForBeaconReason, opv1alpha1.PendingCondition.GetReason(&status))
	assert.True(t, status.TerminatedAt.IsZero())
}

// TestOnChange_MissingClusterStillFailsRunningOperation guards the rule from over-reaching: for an
// operation which still has work to dispatch, a missing cluster is exactly the failure it always
// was.
func TestOnChange_MissingClusterStillFailsRunningOperation(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhasePending,
		opv1alpha1.OperationPhaseInProgress,
	} {
		t.Run(string(phase), func(t *testing.T) {
			op := missingClusterOp(newOp())
			op.Spec.TTL = -1
			op.Status = opv1alpha1.ETCDSnapshotRestoreStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
				Step:            opv1alpha1.ETCDSnapshotRestoreStepRestore,
			}

			h, _, _ := newOnChangeHandler(newBeacon("", false))

			status, err := h.OnChange(op, op.Status)
			assert.NoError(t, err)
			assert.Equal(t, opv1alpha1.OperationPhaseFailed, status.Phase)
			assert.Equal(t, opv1alpha1.ClusterNotFoundReason, opv1alpha1.FailedCondition.GetReason(&status))
			assert.False(t, status.TerminatedAt.IsZero(),
				"there is no beacon to release, so the failure must not be left uncollectable")
		})
	}
}
