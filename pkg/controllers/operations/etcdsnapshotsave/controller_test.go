package etcdsnapshotsave

import (
	"encoding/json"
	"errors"
	"reflect"
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
	corecontrollers "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// stubAdapter is a minimal ops.Adapter implementation for tests. Each field is what the
// corresponding method returns; RenderProbes always returns the same map regardless of secret/
// supervisor flag to keep produced plans byte-deterministic across calls.
type stubAdapter struct {
	runtimeCommand     string
	dataDir            string
	provisioningDir    string
	kubectlPath        string
	kubeconfigPath     string
	serverUnit         string
	waitForRegisterOK  bool
	waitForRegisterErr error
	probes             map[string]planapi.Probe
}

func (a *stubAdapter) BeaconRef() (string, string)   { return "test-namespace", "test-cluster" }
func (a *stubAdapter) EtcdSnapshotNamespace() string { return "test-namespace" }
func (a *stubAdapter) ClusterObject() (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}

// The restore-target and install methods complete the ops.Adapter contract; only the etcd snapshot
// restore controller uses them.
func (a *stubAdapter) RestoreTarget(_ string) (*unstructured.Unstructured, error) { return nil, nil }
func (a *stubAdapter) UpdateRestoreTarget(_ *unstructured.Unstructured) error     { return nil }
func (a *stubAdapter) WaitForRestoreTarget() (bool, error)                        { return true, nil }
func (a *stubAdapter) InstallInstruction(_ *corev1.Secret, _ string) (planapi.OneTimeInstruction, bool) {
	return planapi.OneTimeInstruction{}, false
}
func (a *stubAdapter) WaitForRegister() (bool, error) {
	return a.waitForRegisterOK, a.waitForRegisterErr
}
func (a *stubAdapter) PauseCluster(_ bool) error { return nil }
func (a *stubAdapter) RuntimeCommand() string    { return a.runtimeCommand }
func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) (string, error) {
	return a.dataDir, nil
}
func (a *stubAdapter) DistroManifestPaths(_ string) ops.ManifestPaths {
	return ops.ManifestPaths{}
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
	return a.kubectlPath, nil
}
func (a *stubAdapter) KubeconfigPath(_ *corev1.Secret) string { return a.kubeconfigPath }
func (a *stubAdapter) FindOrElectLeader(_ string, _ ops.Filter) (*corev1.Secret, error) {
	return nil, nil
}

// The six methods below complete the ops.Adapter contract for the stub. They are not exercised
// by the snapshot-save controller (which only consumes runtime/dataDir/serverUnit/probes/plans),
// so each returns a static, runtime-appropriate value.
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

// fakeDynamic satisfies the controller's dynamicResolver interface for the success-path tests.
// Enqueue records the (gvk, namespace, name) tuple so tests can assert handleSucceeded nudged
// the parent cluster.
type fakeDynamic struct {
	gets       map[string]runtime.Object
	enqueued   []string
	getErr     error
	enqueueErr error
}

func (f *fakeDynamic) Get(gvk schema.GroupVersionKind, ns, name string) (runtime.Object, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if obj, ok := f.gets[gvk.String()+"/"+ns+"/"+name]; ok {
		return obj, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, name)
}

func (f *fakeDynamic) Enqueue(gvk schema.GroupVersionKind, ns, name string) error {
	f.enqueued = append(f.enqueued, gvk.String()+"/"+ns+"/"+name)
	return f.enqueueErr
}

// defaultAdapter returns a fully-populated stubAdapter suitable for most reconcile tests. K3s is
// chosen because the runtime is irrelevant to the controller logic; only the rendered command
// strings matter.
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

// newScope wires together the common per-reconcile context for the tests. The ownerKey mirrors
// what the real controller computes in resolveScope (ops.BeaconOwnerKey(OperationKind, op))
// so beacon fixtures created with `testOwnerKey` will match ownership + delegate checks.
func newScope(op *opv1alpha1.ETCDSnapshotSave, beacon *planv1alpha1.Beacon, adapter *stubAdapter) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetName("test")
	cluster.SetNamespace("fleet-default")
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")
	return &scope{
		ownerKey:   ops.BeaconOwnerKey(OperationKind, op),
		op:         op,
		beacon:     beacon,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    adapter,
	}
}

// testOwnerKey is the fully-qualified beacon owner key for the canonical `newOp()` operation.
// Tests that want to build a beacon owned by "us" pass this to newBeacon, instead of the plain
// ControllerOwnerKey prefix which no longer matches what the handler computes at reconcile time.
var testOwnerKey = ops.BeaconOwnerKey(OperationKind, newOp())

func newOp() *opv1alpha1.ETCDSnapshotSave {
	return &opv1alpha1.ETCDSnapshotSave{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "save-1",
			Namespace: "fleet-default",
			UID:       "op-uid",
		},
		Spec: opv1alpha1.ETCDSnapshotSaveSpec{
			OperationSpec: opv1alpha1.OperationSpec{},
		},
	}
}

func newBeacon(owner string, active bool) *planv1alpha1.Beacon {
	// Beacon ownership lives on Status.Owner (the plan.AcquireBeacon helper writes there).
	// We populate the legacy BeaconOwnerLabel too so any caller that still reads it (e.g.
	// EncryptionKeyRotation's reclaimStaleBeaconOwnerIfNeeded) keeps working.
	return &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "fleet-default",
		},
		Status: planv1alpha1.BeaconStatus{
			Active: active,
			Owner:  owner,
		},
	}
}

// newPlanSecret builds a machine-plan secret carrying the cluster-name + etcd-role labels and
// non-nil Annotations (the plan.Store assumes Annotations is non-nil).
func newPlanSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "fleet-default",
			UID:         types.UID(name + "-uid"),
			Annotations: map[string]string{},
			Labels: map[string]string{
				capr.ClusterNameLabel: "test",
				capr.EtcdRoleLabel:    "true",
			},
		},
		Type: planapi.SecretTypeMachinePlan,
	}
}

// withAppliedPlan returns a copy of the secret pre-populated to look like the system-agent has
// already applied the given plan with healthy probes — used to fast-forward reconcile tests
// through the "wait for plan" branch.
func withAppliedPlan(secret *corev1.Secret, expectedPlan *planapi.Plan) *corev1.Secret {
	out := secret.DeepCopy()
	data, _ := json.Marshal(expectedPlan)
	if out.Data == nil {
		out.Data = map[string][]byte{}
	}
	out.Data["plan"] = data
	out.Data["appliedPlan"] = data
	out.Data["probe-statuses"] = []byte(`{"x":{"healthy":true}}`)
	if out.Annotations == nil {
		out.Annotations = map[string]string{}
	}
	out.Annotations[planapi.PlanProbesPassedAnnotation] = "applied"
	return out
}

// withFailedPlan marks the secret as having failed its plan past the failure threshold so the
// store reports Failure() == true.
func withFailedPlan(secret *corev1.Secret, expectedPlan *planapi.Plan) *corev1.Secret {
	out := secret.DeepCopy()
	data, _ := json.Marshal(expectedPlan)
	if out.Data == nil {
		out.Data = map[string][]byte{}
	}
	out.Data["plan"] = data
	out.Data["failed-checksum"] = []byte(planapi.PlanHash(data))
	out.Data["failure-count"] = []byte("5")
	out.Data["max-failures"] = []byte("1")
	out.Data["failure-threshold"] = []byte("1")
	return out
}

// newSecretClient mocks the SecretClient used by planapi.Store.AssignPlan. Update echoes the
// passed-in secret back to the caller so the store treats it as the "post-update" state.
func newSecretClient(t *testing.T, ctrl *gomock.Controller, items ...*corev1.Secret) *ctrlfake.MockClientInterface[*corev1.Secret, *corev1.SecretList] {
	t.Helper()
	m := ctrlfake.NewMockClientInterface[*corev1.Secret, *corev1.SecretList](ctrl)
	m.EXPECT().Update(gomock.Any()).DoAndReturn(func(s *corev1.Secret) (*corev1.Secret, error) {
		return s, nil
	}).AnyTimes()
	m.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(ns string, opts metav1.ListOptions) (*corev1.SecretList, error) {
		sel, err := labels.Parse(opts.LabelSelector)
		if err != nil {
			return nil, err
		}
		var out corev1.SecretList
		for _, s := range items {
			if s.Namespace != ns {
				continue
			}
			if !sel.Matches(labels.Set(s.Labels)) {
				continue
			}
			out.Items = append(out.Items, *s)
		}
		return &out, nil
	}).AnyTimes()
	return m
}

// fakeBeaconClient is a tiny in-memory implementation of plancontrollers.BeaconClient covering
// only Update and UpdateStatus — the only methods AcquireBeacon/ReleaseBeacon/ToggleBeacon call.
// Building a gomock interface for the full ClientInterface would dwarf the controller logic
// under test; this hand-written stub keeps the assertions readable.
type fakeBeaconClient struct {
	plancontrollers.BeaconClient // embed for any unused method; nil panics signal an unexpected call

	// beacon is what Get serves, and is kept in step with UpdateStatus so a handler driven over
	// several reconciles observes its own beacon writes. A nil beacon makes Get report NotFound.
	beacon *planv1alpha1.Beacon

	updates       []*planv1alpha1.Beacon
	statusUpdates []*planv1alpha1.Beacon
	updateErr     error
	// events, when set, records beacon writes into a log shared with other fakes, so a test can
	// assert the order work happened in rather than just that it happened.
	events *[]string
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
	return updated, f.updateErr
}

func (f *fakeBeaconClient) UpdateStatus(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	if f.events != nil {
		*f.events = append(*f.events, "beacon-write")
	}
	f.statusUpdates = append(f.statusUpdates, b.DeepCopy())
	f.beacon = b.DeepCopy()
	return b, nil
}

type fakeETCDSnapshotSaveController struct {
	operationcontrollers.ETCDSnapshotSaveController
	enqueueCalls int
	deleteCalls  int
	// updates records the objects passed to Update — the finalizer is the only thing the handler
	// writes outside of status, so each entry is a finalizer add or removal.
	updates   []*opv1alpha1.ETCDSnapshotSave
	updateErr error
}

func (f *fakeETCDSnapshotSaveController) EnqueueAfter(_, _ string, _ time.Duration) {
	f.enqueueCalls++
}

func (f *fakeETCDSnapshotSaveController) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	f.deleteCalls++
	return nil
}

func (f *fakeETCDSnapshotSaveController) Update(op *opv1alpha1.ETCDSnapshotSave) (*opv1alpha1.ETCDSnapshotSave, error) {
	f.updates = append(f.updates, op.DeepCopy())
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return op, nil
}

// newDeletingOp returns an operation which has been deleted and still carries our finalizer — the
// state the API server leaves an in-flight operation in until the controller releases it.
func newDeletingOp() *opv1alpha1.ETCDSnapshotSave {
	op := newOp()
	op.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	op.Finalizers = []string{Finalizer}
	return op
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

// withClusterRef points the operation at a cluster of testClusterGVK. Tests that pass a fakeDynamic
// without the matching object exercise the cluster-is-gone paths.
func withClusterRef(op *opv1alpha1.ETCDSnapshotSave, name string) *opv1alpha1.ETCDSnapshotSave {
	op.Spec.ClusterRef = &corev1.ObjectReference{
		APIVersion: testClusterGVK.GroupVersion().String(),
		Kind:       testClusterGVK.Kind,
		Namespace:  "fleet-default",
		Name:       name,
	}
	return op
}

// newOnChangeHandler wires a handler for the full OnChange path: the cluster referenced by
// withClusterRef(op, "test") resolves through the dynamic resolver, and the beacon lookup serves
// (and mutates) the given beacon. A nil beacon makes the lookup report NotFound.
func newOnChangeHandler(beacon *planv1alpha1.Beacon) (*handler, *fakeETCDSnapshotSaveController, *fakeBeaconClient) {
	controller := &fakeETCDSnapshotSaveController{}
	beacons := &fakeBeaconClient{beacon: beacon}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(testClusterGVK)
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test")

	h := &handler{
		etcdsnapshotsaves: controller,
		beacons:           beacons,
		dynamic: &fakeDynamic{gets: map[string]runtime.Object{
			testClusterGVK.String() + "/fleet-default/test": cluster,
		}},
	}

	return h, controller, beacons
}

// --- updateStatus ---------------------------------------------------------------------------

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

			initialStatus := opv1alpha1.ETCDSnapshotSaveStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase: opv1alpha1.OperationPhaseInProgress,
				},
				Step: opv1alpha1.ETCDSnapshotSaveStepSave,
			}

			if tc.initiallyPaused {
				opv1alpha1.PausedCondition.True(&initialStatus)
				opv1alpha1.PausedCondition.Reason(&initialStatus, opv1alpha1.PausedReason)
				opv1alpha1.PausedCondition.Message(&initialStatus, "Operation is paused")
			}

			status := updateStatus(op, initialStatus)

			assert.Equal(t, int64(7), status.ObservedGeneration, "ObservedGeneration must be copied from the op")
			assert.Equal(t, tc.expectedStatus, opv1alpha1.PausedCondition.GetStatus(&status))
			assert.Equal(t, tc.expectedReason, opv1alpha1.PausedCondition.GetReason(&status))
			assert.Equal(t, tc.expectedMessage, opv1alpha1.PausedCondition.GetMessage(&status))

			// Verify phase and step are unchanged
			assert.Equal(t, initialStatus.Phase, status.Phase, "Phase should be unchanged")
			assert.Equal(t, initialStatus.Step, status.Step, "Step should be unchanged")
		})
	}
}

func TestUpdateStatusByPhase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		phase opv1alpha1.OperationPhase
		check func(t *testing.T, status opv1alpha1.ETCDSnapshotSaveStatus)
	}{
		{
			name:  "pending sets Pending=True",
			phase: opv1alpha1.OperationPhasePending,
			check: func(t *testing.T, s opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, "True", opv1alpha1.PendingCondition.GetStatus(&s))
			},
		},
		{
			name:  "in-progress clears Pending",
			phase: opv1alpha1.OperationPhaseInProgress,
			check: func(t *testing.T, s opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, "False", opv1alpha1.PendingCondition.GetStatus(&s))
				assert.Equal(t, opv1alpha1.InProgressReason, opv1alpha1.PendingCondition.GetReason(&s))
			},
		},
		{
			// The outcome is asserted as soon as the phase is reached; only Finalized waits for
			// the controller to be done with the operation.
			name:  "succeeded asserts the outcome and finalizes",
			phase: opv1alpha1.OperationPhaseSucceeded,
			check: func(t *testing.T, s opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, "True", opv1alpha1.SucceededCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.FailedCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.InProgressCondition.GetStatus(&s))
				assert.Equal(t, opv1alpha1.FinalizingReason, opv1alpha1.InProgressCondition.GetReason(&s))
				assert.Equal(t, "False", opv1alpha1.FinalizedCondition.GetStatus(&s))
				assert.Equal(t, opv1alpha1.FinalizingReason, opv1alpha1.FinalizedCondition.GetReason(&s))
			},
		},
		{
			name:  "failed asserts the outcome and finalizes",
			phase: opv1alpha1.OperationPhaseFailed,
			check: func(t *testing.T, s opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, "True", opv1alpha1.FailedCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.SucceededCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.InProgressCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.FinalizedCondition.GetStatus(&s))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			op := newOp()
			status := updateStatus(op, opv1alpha1.ETCDSnapshotSaveStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: tc.phase},
			})
			tc.check(t, status)
		})
	}
}

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

			initial := opv1alpha1.ETCDSnapshotSaveStatus{
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
			got := updateStatus(newOp(), opv1alpha1.ETCDSnapshotSaveStatus{
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

func TestHandlePending_NilBeacon(t *testing.T) {
	t.Parallel()

	h := &handler{beacons: &fakeBeaconClient{}}
	s := newScope(newOp(), nil, defaultAdapter())

	got, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// AcquireBeacon returns nil when beacon is nil → handler keeps Pending and reports waiting.
	assert.Empty(t, string(got.Phase), "no phase set yet")
	assert.Equal(t, opv1alpha1.WaitingForBeaconReason, opv1alpha1.PendingCondition.GetReason(&got))
}

func TestHandlePending_BeaconOwnedByOther(t *testing.T) {
	t.Parallel()

	h := &handler{beacons: &fakeBeaconClient{}}
	s := newScope(newOp(), newBeacon("some-other-controller", false), defaultAdapter())

	got, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// AcquireBeacon returns nil when another controller owns it → still waiting.
	assert.Empty(t, string(got.Phase))
	assert.Equal(t, opv1alpha1.WaitingForBeaconReason, opv1alpha1.PendingCondition.GetReason(&got))
}

func TestHandlePending_WaitingForRegistration(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	adapter := defaultAdapter()
	adapter.waitForRegisterOK = false

	h := &handler{beacons: beacons}
	s := newScope(newOp(), newBeacon(testOwnerKey, false), adapter)

	got, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Empty(t, string(got.Phase), "phase must not advance until all agents have registered")
	assert.Equal(t, opv1alpha1.WaitingForRegistrationReason, opv1alpha1.PendingCondition.GetReason(&got))
}

func TestHandlePending_TransitionsToInProgress(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	h := &handler{beacons: beacons}
	a := defaultAdapter()
	a.waitForRegisterOK = true
	s := newScope(newOp(), newBeacon(testOwnerKey, false), a)

	got, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepPreflight, got.Step)
}

func TestHandlePending_WaitForRegisterErrorBubbles(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("kaboom")
	adapter := defaultAdapter()
	adapter.waitForRegisterErr = sentinel
	adapter.waitForRegisterOK = false

	h := &handler{beacons: &fakeBeaconClient{}}
	_, err := h.handlePending(newScope(newOp(), newBeacon(testOwnerKey, false), adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.ErrorIs(t, err, sentinel)
}

func TestHandleInProgress_BeaconLost(t *testing.T) {
	t.Parallel()

	h := &handler{beacons: &fakeBeaconClient{}}
	// Beacon owned by some other controller → handler must fail rather than continue.
	s := newScope(newOp(), newBeacon("other", true), defaultAdapter())

	got, err := h.handleInProgress(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.BeaconLostReason, opv1alpha1.FailedCondition.GetReason(&got))
}

func TestHandleInProgress_UnknownStep(t *testing.T) {
	t.Parallel()

	h := &handler{beacons: &fakeBeaconClient{}}
	op := newOp()
	op.Status.Step = "Whatever"
	s := newScope(op, newBeacon(testOwnerKey, false), defaultAdapter())

	got, err := h.handleInProgress(s, opv1alpha1.ETCDSnapshotSaveStatus{Step: "Whatever"})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.UnknownStepReason, opv1alpha1.FailedCondition.GetReason(&got))
}

func TestHandleFailed_HoldingBeaconReleases(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	h := &handler{beacons: beacons}
	s := newScope(newOp(), newBeacon(testOwnerKey, true), defaultAdapter())

	_, err := h.handleFailed(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// ReleaseBeacon on the owner path clears Active + Owner + Delegates in a single UpdateStatus
	// call, so we expect exactly one status update and no main-resource update.
	if assert.Len(t, beacons.statusUpdates, 1, "ReleaseBeacon should update the beacon status") {
		assert.False(t, beacons.statusUpdates[0].Status.Active, "beacon must be toggled inactive on release")
		assert.Equal(t, "", beacons.statusUpdates[0].Status.Owner, "Status.Owner must be cleared on release")
	}
	assert.Empty(t, beacons.updates, "ReleaseBeacon no longer touches the main resource")
}

func TestHandleFailed_NotHoldingNoOp(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	h := &handler{beacons: beacons}
	s := newScope(newOp(), newBeacon("other", true), defaultAdapter())

	_, err := h.handleFailed(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Empty(t, beacons.updates, "non-owners must not touch the beacon")
	assert.Empty(t, beacons.statusUpdates)
}

func TestHandleSucceeded_NotHoldingNoOp(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	dyn := &fakeDynamic{}
	h := &handler{beacons: beacons, dynamic: dyn}
	s := newScope(newOp(), newBeacon("other", true), defaultAdapter())

	_, err := h.handleSucceeded(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Empty(t, beacons.updates)
	assert.Empty(t, dyn.enqueued, "non-owners must not enqueue the cluster")
}

func TestHandleSucceeded_HoldingBeaconEnqueuesCluster(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	dyn := &fakeDynamic{}
	h := &handler{beacons: beacons, dynamic: dyn}
	s := newScope(newOp(), newBeacon(testOwnerKey, true), defaultAdapter())

	_, err := h.handleSucceeded(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// ReleaseBeacon on the owner path clears Active + Owner + Delegates in a single UpdateStatus
	// call.
	if assert.Len(t, beacons.statusUpdates, 1, "ReleaseBeacon should update the beacon status") {
		assert.False(t, beacons.statusUpdates[0].Status.Active, "beacon must be toggled inactive on release")
		assert.Equal(t, "", beacons.statusUpdates[0].Status.Owner, "Status.Owner must be cleared on release")
	}
	assert.Empty(t, beacons.updates, "ReleaseBeacon no longer touches the main resource")
	if assert.Len(t, dyn.enqueued, 1, "parent cluster must be re-enqueued") {
		assert.Equal(t, "provisioning.cattle.io/v1, Kind=Cluster/fleet-default/test", dyn.enqueued[0])
	}
}

// expectedSaveInstruction builds the snapshot save instruction the controller will dispatch given
// an op spec and stubAdapter, so tests can predict the exact plan bytes the agent will see.
func expectedSaveInstruction(op *opv1alpha1.ETCDSnapshotSave, runtime string) planapi.OneTimeInstruction {
	args := []string{"etcd-snapshot", "save"}
	if op.Spec.Args.Name != "" {
		args = append(args, "--name", op.Spec.Args.Name)
	}
	return planapi.OneTimeInstruction{
		CommonInstruction: planapi.CommonInstruction{
			Name:    "snapshot",
			Command: runtime,
			Args:    args,
		},
	}
}

// Both plans are scoped to the operation and step they belong to, exactly as the controller assigns
// them: without that the plan bytes of two operations would be identical, and the second would be
// reported as already applied instead of being executed.
func expectedSavePlan(op *opv1alpha1.ETCDSnapshotSave, adapter *stubAdapter) *planapi.Plan {
	return ops.WithOperationEnv(&planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{expectedSaveInstruction(op, adapter.runtimeCommand)},
		Probes:              adapter.probes,
	}, ops.OperationEnv(ControllerOwnerKey, op, opv1alpha1.ETCDSnapshotSaveStepSave))
}

func expectedRestartPlan(op *opv1alpha1.ETCDSnapshotSave, adapter *stubAdapter) *planapi.Plan {
	return ops.WithOperationEnv(&planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			{CommonInstruction: planapi.CommonInstruction{
				Name:    "restart",
				Command: "systemctl",
				Args:    []string{"restart", adapter.serverUnit},
			}},
		},
		Probes: adapter.probes,
	}, ops.OperationEnv(ControllerOwnerKey, op, opv1alpha1.ETCDSnapshotSaveStepRestart))
}

func TestReconcileSave_NoSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	h := &handler{
		secrets: newSecretClient(t, ctrl),
	}
	h.store = planapi.NewStore(h.secrets)

	status, err := h.reconcileSave(newScope(newOp(), nil, defaultAdapter()), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepSave})
	// The Collector validator surfaces the empty-set condition as an error; the outer status
	// handler will requeue (and the op stays in its current phase until the situation resolves).
	assert.NoError(t, err, "terminal errors should not trigger reenqueue")
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, status.Phase, "terminal errors should cause operation to fail")
}

func TestReconcileSave_WaitsForPlanApply(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := newPlanSecret("etcd-1") // no plan applied yet → first dispatch
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepSave})
	assert.NoError(t, err)
	// Plan was just delivered to the agent — controller must report InProgress and let the next
	// reconcile poll feedback.
	assert.Empty(t, string(got.Phase), "phase must not advance while a plan is pending")
	assert.Equal(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&got))
}

func TestReconcileSave_TransitionsToRestartWhenApplied(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := withAppliedPlan(newPlanSecret("etcd-1"), expectedSavePlan(op, adapter))
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepSave})
	assert.NoError(t, err)
	assert.Empty(t, string(got.Phase), "phase must not change on a clean transition")
	assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepRestart, got.Step)
}

func TestReconcileSave_PlanFailureMarksFailed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := withFailedPlan(newPlanSecret("etcd-1"), expectedSavePlan(op, adapter))
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepSave})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.PlanFailedReason, opv1alpha1.FailedCondition.GetReason(&got))
}

func TestReconcileSave_AppliesSnapshotArgs(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	op.Spec.Args.Name = "my-snap"
	adapter := defaultAdapter()

	// Pre-populate so the test traverses the "applied" branch without needing additional poll
	// cycles — we're asserting on the *plan content* not the wait behavior here.
	expectedPlan := expectedSavePlan(op, adapter)
	secret := withAppliedPlan(newPlanSecret("etcd-1"), expectedPlan)
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	_, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepSave})
	assert.NoError(t, err)

	wantArgs := []string{"etcd-snapshot", "save", "--name", "my-snap"}
	if !reflect.DeepEqual(expectedPlan.OneTimeInstructions[0].Args, wantArgs) {
		t.Errorf("plan args = %v, want %v — snapshot Args were not threaded through", expectedPlan.OneTimeInstructions[0].Args, wantArgs)
	}
}

func TestReconcileRestart_MarksSucceededWhenApplied(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := withAppliedPlan(newPlanSecret("etcd-1"), expectedRestartPlan(op, adapter))
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, got.Phase)
	assert.Equal(t, opv1alpha1.FinishedReason, opv1alpha1.SucceededCondition.GetReason(&got))
}

func TestReconcileRestart_WaitsForPlanApply(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := newPlanSecret("etcd-1") // no plan applied yet
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart})
	assert.NoError(t, err)
	assert.Empty(t, string(got.Phase), "phase must not advance to Succeeded while restart is pending")
	assert.Equal(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&got))
}

func TestReconcileRestart_PlanFailureMarksFailed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := withFailedPlan(newPlanSecret("etcd-1"), expectedRestartPlan(op, adapter))
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.PlanFailedReason, opv1alpha1.FailedCondition.GetReason(&got))
}

func TestReconcileRestart_FiltersToEtcdSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	etcd := withAppliedPlan(newPlanSecret("etcd-1"), expectedRestartPlan(op, adapter))
	worker := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "worker-1",
			Namespace:   "fleet-default",
			UID:         "worker-uid",
			Annotations: map[string]string{},
			Labels: map[string]string{
				capr.ClusterNameLabel: "test",
				capr.WorkerRoleLabel:  "true",
			},
		},
		Type: planapi.SecretTypeMachinePlan,
	}
	h := &handler{
		secrets: newSecretClient(t, ctrl, etcd, worker),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart})
	assert.NoError(t, err)
	// Worker secret must be ignored — only etcd nodes receive the restart plan; success would
	// not be reached if the worker were included (its plan is not in "applied" state).
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, got.Phase, "non-etcd secrets must not be in the iteration")
}

// TestAssignedPlansAreOperationScoped covers the property the assigned plans depend on: AssignPlan
// only writes a plan whose bytes differ from the one already on the secret, and the system-agent only
// re-runs a plan whose content changed. Two saves of the same shape must therefore serialize
// differently, otherwise a save retried after a failed one would be reported as already applied and
// succeed without ever taking a snapshot.
func TestAssignedPlansAreOperationScoped(t *testing.T) {
	t.Parallel()

	adapter := defaultAdapter()

	opWithUID := func(uid types.UID) *opv1alpha1.ETCDSnapshotSave {
		op := newOp()
		op.UID = uid
		return op
	}

	for name, build := range map[string]func(*opv1alpha1.ETCDSnapshotSave) *planapi.Plan{
		"save":    func(op *opv1alpha1.ETCDSnapshotSave) *planapi.Plan { return expectedSavePlan(op, adapter) },
		"restart": func(op *opv1alpha1.ETCDSnapshotSave) *planapi.Plan { return expectedRestartPlan(op, adapter) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			marshal := func(uid types.UID) string {
				data, err := json.Marshal(build(opWithUID(uid)))
				assert.NoError(t, err)
				return string(data)
			}

			assert.NotEqual(t, marshal("save-uid-1"), marshal("save-uid-2"),
				"plans for two operations must not serialize identically, or the second is reported as already applied")
			assert.Equal(t, marshal("save-uid-1"), marshal("save-uid-1"),
				"plans for one operation must serialize identically across reconciles")
		})
	}
}

func TestOnChange_StablePausedOperation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		phase       opv1alpha1.OperationPhase
		step        opv1alpha1.ETCDSnapshotSaveStep
		ttl         int64
		lastUpdated metav1.Time
	}{
		{
			name:        "stable in-progress paused operation",
			phase:       opv1alpha1.OperationPhaseInProgress,
			step:        opv1alpha1.ETCDSnapshotSaveStepSave,
			ttl:         300,
			lastUpdated: metav1.Now(),
		},
		{
			name:        "stable terminal expired paused operation",
			phase:       opv1alpha1.OperationPhaseSucceeded,
			step:        opv1alpha1.ETCDSnapshotSaveStepSave,
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

			initialStatus := opv1alpha1.ETCDSnapshotSaveStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase:       tc.phase,
					LastUpdated: tc.lastUpdated,
				},
				Step: tc.step,
			}

			// Pre-compute the expected status with paused condition
			currentStatus := updateStatus(op, initialStatus)
			op.Status = currentStatus

			controller := &fakeETCDSnapshotSaveController{}
			h := &handler{
				etcdsnapshotsaves: controller,
			}

			returnedStatus, err := h.OnChange(op, op.Status)
			if err != nil {
				t.Fatalf("OnChange returned error: %v", err)
			}

			// Verify status unchanged
			if !equality.Semantic.DeepEqual(returnedStatus, currentStatus) {
				t.Errorf("returnedStatus differs from currentStatus")
			}

			// Verify phase and step preserved
			assert.Equal(t, tc.phase, returnedStatus.Phase)
			assert.Equal(t, tc.step, returnedStatus.Step)

			// Verify no delete occurred
			assert.Zero(t, controller.deleteCalls, "Delete should not be called")

			// Verify no enqueue occurred
			assert.Zero(t, controller.enqueueCalls, "EnqueueAfter should not be called")
		})
	}
}

func TestOnChange_Paused(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true
	op.Generation = 7

	initialStatus := opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase: opv1alpha1.OperationPhaseInProgress,
		},
		Step: opv1alpha1.ETCDSnapshotSaveStepSave,
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
	handle func(*handler, *scope, opv1alpha1.ETCDSnapshotSaveStatus) (opv1alpha1.ETCDSnapshotSaveStatus, error)
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
			s := newScope(newOp(), newBeacon(testOwnerKey, true), defaultAdapter())

			got, err := tc.handle(h, s, opv1alpha1.ETCDSnapshotSaveStatus{})
			assert.NoError(t, err)
			assert.False(t, got.TerminatedAt.IsZero(),
				"terminal handling completed (beacon released), so it must be recorded on the status")
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
			s := newScope(op, newBeacon(testOwnerKey, true), defaultAdapter())

			// The outcome the phase handler recorded before delegating. It is the only record of
			// why the operation ended, so delegating must not overwrite it — the delegate is
			// reported on Finalized by updateStatus instead.
			status := opv1alpha1.ETCDSnapshotSaveStatus{}
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

	op := withClusterRef(newDeletingOp(), "test")
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
func TestOnChange_DeletionCancelsInFlightOperation(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newDeletingOp(), "test")
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

	op := withClusterRef(newDeletingOp(), "test")
	op.Labels = map[string]string{opv1alpha1.CanceledPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

	op := withClusterRef(newDeletingOp(), "test")
	op.Status = updateStatus(op, opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:        opv1alpha1.OperationPhaseSucceeded,
			TerminatedAt: metav1.Now(),
		},
		Step: opv1alpha1.ETCDSnapshotSaveStepRestart,
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

	op := withClusterRef(newDeletingOp(), "test")
	op.Spec.Paused = true
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

	op := withClusterRef(newDeletingOp(), "gone")
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

	op := withClusterRef(newDeletingOp(), "test")
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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
	op := withClusterRef(newOp(), "test")

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

	op := withClusterRef(newOp(), "test")
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

	op := withClusterRef(newOp(), "test")
	op.Finalizers = []string{Finalizer}
	op.Spec.TTL = 0 // expire as soon as the operation is terminal
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:       opv1alpha1.OperationPhaseSucceeded,
			LastUpdated: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Step: opv1alpha1.ETCDSnapshotSaveStepRestart,
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

// --- cancellation ----------------------------------------------------------------------------

// TestOnChange_CancelRequestedCancelsInFlightOperation covers the core of the cancellation
// contract, which mirrors the deletion one: an operation canceled while it is still running stops
// where it is, releases the beacon, and reports why it was canceled.
func TestOnChange_CancelRequestedCancelsInFlightOperation(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newOp(), "test")
	op.Spec.Cancel = true
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

// TestOnChange_CancelRequestedInTerminalPhaseIsDeclined covers the edge of the window. The
// operation has reached a terminal phase but is still waiting on that phase's lifecycle hook, so it
// is holding the beacon on a delegate's behalf — yet its work is over and its outcome asserted, so
// there is nothing for a cancellation to stop. The request is declined and reported on the Canceled
// condition, and the hook keeps the beacon; deleting the operation is what breaks that deadlock, as
// TestOnChange_DeletionCancelsTerminalPhaseWaitingOnHook covers.
func TestOnChange_CancelRequestedInTerminalPhaseIsDeclined(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newOp(), "test")
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	// The operation succeeded, then handed the beacon to a delegate that never gave it back.
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "wedged": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.ETCDSnapshotSaveStepRestart,
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
	assert.Equal(t, opv1alpha1.WaitingForDelegateReason, opv1alpha1.FinalizedCondition.GetReason(&status),
		"Finalized is what names the delegate holding the operation up")
	for _, update := range beacons.statusUpdates {
		assert.Equal(t, testOwnerKey, update.Status.Owner, "the beacon must stay with the operation and its delegate")
	}
}

// TestOnChange_DeletionCancelsTerminalPhaseWaitingOnHook is the counterpart, and the reason cancel
// and delete answer this window differently. A deleted operation has to release the beacon and
// retire its finalizer whatever phase it is in — otherwise it would wait forever on a hook nothing
// will answer, and never finish deleting — so deletion cancels where cancellation declines.
func TestOnChange_DeletionCancelsTerminalPhaseWaitingOnHook(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newDeletingOp(), "test")
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "wedged": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.ETCDSnapshotSaveStepRestart,
	}

	beacon := newBeacon(testOwnerKey, true)
	beacon.Status.Delegates = []string{"delegate-a"}
	h, _, beacons := newOnChangeHandler(beacon)

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseCanceled, status.Phase)
	assert.Equal(t, opv1alpha1.OperationDeletedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	assert.False(t, status.TerminatedAt.IsZero())
	if assert.Len(t, beacons.statusUpdates, 1, "the beacon must be freed despite the abandoned hook") {
		assert.Equal(t, "", beacons.statusUpdates[0].Status.Owner)
		assert.Empty(t, beacons.statusUpdates[0].Status.Delegates,
			"the delegate the abandoned hook pushed must not be left on the beacon")
	}
}

// TestOnChange_CancelRequestedAfterTerminationKeepsOutcome is the same rule for an operation the
// controller has fully finished with: nothing left to call off, and by then nothing left to release
// either.
func TestOnChange_CancelRequestedAfterTerminationKeepsOutcome(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newOp(), "test")
	op.Spec.Cancel = true
	// A TTL that never expires, so the terminal operation lingers instead of being collected —
	// what is under test is the phase, not the garbage collection that eventually removes it.
	op.Spec.TTL = -1
	op.Status = updateStatus(op, opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:        opv1alpha1.OperationPhaseSucceeded,
			TerminatedAt: metav1.Now(),
		},
		Step: opv1alpha1.ETCDSnapshotSaveStepRestart,
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

	op := withClusterRef(newOp(), "test")
	op.Spec.Paused = true
	op.Spec.Cancel = true
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

// TestHandleTerminal_WithoutBeaconClaimLeavesItUntouched covers every outcome an operation can
// reach without holding the beacon — Failed after losing it, Aborted after being overtaken,
// Canceled by whoever wanted it next. In all three the operation still finishes, and the beacon
// (now someone else's) is left exactly as it is: not cleared, and not carrying the phase hook's
// delegate, which is the write that would otherwise reach into another controller's operation.
func TestHandleTerminal_WithoutBeaconClaimLeavesItUntouched(t *testing.T) {
	t.Parallel()

	for name, tc := range terminalHandlers {
		if name == "succeeded" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			op := newOp()
			// A hook on the phase being handled, which must be passed over rather than delegated:
			// with no claim on the beacon there is no authority to hand to a delegate.
			op.Labels = map[string]string{tc.hook + "cleanup": "delegate-a"}

			beacons := &fakeBeaconClient{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(op, newBeacon("another-controller", true), defaultAdapter())

			got, err := tc.handle(h, s, opv1alpha1.ETCDSnapshotSaveStatus{})
			assert.NoError(t, err)
			assert.False(t, got.TerminatedAt.IsZero(),
				"with no beacon to release, terminal handling is trivially complete")
			assert.Empty(t, beacons.statusUpdates, "a beacon held by another controller must not be modified")
			assert.Empty(t, beacons.updates)
		})
	}
}

// Succeeded is deliberately excluded: an operation cannot have finished its work without holding
// the beacon throughout, so a missing claim there is an anomaly rather than a state to paper over
// by declaring the operation finished.
func TestHandleSucceeded_WithoutBeaconClaimStillHonorsItsHook(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "cleanup": "delegate-a"}

	beacons := &fakeBeaconClient{}
	h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
	s := newScope(op, newBeacon("another-controller", true), defaultAdapter())

	got, err := h.handleSucceeded(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.True(t, got.TerminatedAt.IsZero(), "the hook is still owed an answer, so handling is not complete")
	assert.NotEmpty(t, beacons.statusUpdates, "the hook's delegate is pushed as usual")
}

// TestHandleTerminal_WithoutBeaconClaimReleasesForDeletion ties the rule to why it matters: an
// operation that lost its beacon and is on its way out records termination, which is the marker
// reconcileDeleting waits on before it retires the finalizer. Without it such an operation would hold
// its finalizer forever and never finish deleting.
func TestHandleTerminal_WithoutBeaconClaimReleasesForDeletion(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newDeletingOp(), "test")
	op.Labels = map[string]string{opv1alpha1.FailedPhaseHookLabelPrefix + "cleanup": "delegate-a"}
	// Pre-computed through updateStatus so the status is already settled: reconcileDeleting retires the
	// finalizer only once the terminal status has been persisted for observers to see.
	op.Status = updateStatus(op, opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:        opv1alpha1.OperationPhaseFailed,
			TerminatedAt: metav1.Now(),
		},
		Step: opv1alpha1.ETCDSnapshotSaveStepSave,
	})
	// The operation failed because the beacon was lost, and it is now held by another controller.
	h, controller, beacons := newOnChangeHandler(newBeacon("another-controller", true))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, status.Phase, "a terminated operation keeps the phase it ended in")
	assert.Empty(t, beacons.statusUpdates, "the beacon belongs to another controller and must not be touched")
	if assert.Len(t, controller.updates, 1, "termination was already recorded, so the finalizer can go") {
		assert.NotContains(t, controller.updates[0].Finalizers, Finalizer)
	}
}

// TestUpdateStatusReportsDeclinedCancellation covers the acknowledgement on its own, across every
// outcome an operation can end in. The Canceled condition is where an observer looks to find out
// what became of a cancellation, so a request that could not be acted on has to say so there —
// leaving the generic denial would make setting spec.Cancel indistinguishable from never having
// set it.
func TestUpdateStatusReportsDeclinedCancellation(t *testing.T) {
	t.Parallel()

	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
	} {
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			initial := opv1alpha1.ETCDSnapshotSaveStatus{
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

		status := opv1alpha1.ETCDSnapshotSaveStatus{
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
	superseded := ops.BeaconOwnerKey(OperationKind, &opv1alpha1.ETCDSnapshotSave{
		ObjectMeta: metav1.ObjectMeta{Namespace: op.Namespace, Name: op.Name, UID: "dead-uid"},
	})

	beacon := newBeacon(superseded, true)
	beacon.Status.Delegates = []string{"dead-delegate"}

	beacons := &fakeBeaconClient{beacon: beacon}
	h := &handler{beacons: beacons}
	s := newScope(op, beacon, defaultAdapter())

	_, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Equal(t, testOwnerKey, s.beacon.Status.Owner, "the operation must end up holding the beacon")
	assert.Empty(t, s.beacon.Status.Delegates, "the dead claim's delegate must not be inherited")
}

// --- a missing cluster or beacon for an operation which has already concluded ------------------

// TestOnChange_CancelWithMissingClusterKeepsOutcome covers the sequence that motivated this rule:
// cancellation is applied before the scope is resolved, so a canceled operation whose cluster is
// gone reaches the missing-cluster branch already terminal. Failing it there would report the
// user's cancellation as a failure.
func TestOnChange_CancelWithMissingClusterKeepsOutcome(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newOp(), "gone")
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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
			t.Parallel()

			op := withClusterRef(newOp(), "gone")
			op.Spec.TTL = -1

			initial := opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart}
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
// TestOnChange_MissingClusterAbandonsOwedHook covers the operation whose cluster is deleted while a
// terminal phase hook is still owed a delegate. The hook cannot be honored — delegation is a push
// onto the beacon's delegate chain and the beacon is reached through the cluster's adapter — so it
// is abandoned rather than waited on, and the second pass proves the operation is collected instead
// of being pinned by a label nobody is coming back to clear.
func TestOnChange_MissingClusterAbandonsOwedHook(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newOp(), "gone")
	op.Spec.TTL = 0
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}

	initial := opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart}
	initial.MarkSucceeded()
	op.Status = initial

	h, controller, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, status.Phase, "the outcome it reached still stands")
	assert.False(t, status.TerminatedAt.IsZero(), "nothing can ever satisfy the hook, so the controller is done")
	assert.Equal(t, "True", opv1alpha1.FinalizedCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.HookAbandonedReason, opv1alpha1.FinalizedCondition.GetReason(&status),
		"the abandoned hook must be reported rather than looking like a clean finish")
	assert.True(t, ops.Collectable(&op.Spec.OperationSpec, &status.OperationStatus),
		"an expired operation nothing is owed on must be collectable")

	// The label is still on the operation, which is what used to pin it here for good.
	op.Status = status
	_, err = h.OnChange(op, op.Status)
	assert.ErrorIs(t, err, generic.ErrSkip)
	assert.Equal(t, 1, controller.deleteCalls, "the operation must be collected, not leaked")
}

// A deleting operation is the deliberate exception: it is being discarded and its hooks go with it,
// so it terminates at once rather than holding its finalizer open for a delegate that will never
// be handed anything.
func TestOnChange_DeletionWithMissingClusterAbandonsOwedHook(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newDeletingOp(), "gone")
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.ETCDSnapshotSaveStepRestart,
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
		// labels the operation carries, for the cases where a lifecycle hook is still owed one.
		labels map[string]string

		wantErr             bool
		wantPhase           opv1alpha1.OperationPhase
		wantReason          string
		wantTerminated      bool
		wantFinalizedReason string
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
		{
			// The beacon this hook would have been delegated on is gone, so nothing can ever
			// satisfy it: waiting on the label would leave the operation un-terminated for good,
			// and out of reach of TTL collection with it.
			name:                "canceled with an owed hook abandons it",
			phase:               opv1alpha1.OperationPhaseCanceled,
			labels:              map[string]string{opv1alpha1.CanceledPhaseHookLabelPrefix + "verify": "delegate-a"},
			wantPhase:           opv1alpha1.OperationPhaseCanceled,
			wantTerminated:      true,
			wantFinalizedReason: opv1alpha1.HookAbandonedReason,
		},
		{
			// Same for the Failed-phase hook of an operation the missing beacon has just failed.
			name:                "in progress with an owed failed hook abandons it",
			phase:               opv1alpha1.OperationPhaseInProgress,
			labels:              map[string]string{opv1alpha1.FailedPhaseHookLabelPrefix + "verify": "delegate-a"},
			wantPhase:           opv1alpha1.OperationPhaseFailed,
			wantReason:          opv1alpha1.BeaconLostReason,
			wantTerminated:      true,
			wantFinalizedReason: opv1alpha1.HookAbandonedReason,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			op := withClusterRef(newOp(), "test")
			op.Spec.TTL = 0
			op.Labels = tc.labels

			initial := opv1alpha1.ETCDSnapshotSaveStatus{Step: opv1alpha1.ETCDSnapshotSaveStepRestart}
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
				"termination is recorded only when nothing can still be owed")
			if tc.wantFinalizedReason != "" {
				assert.Equal(t, tc.wantFinalizedReason, opv1alpha1.FinalizedCondition.GetReason(&status),
					"an abandoned hook must be reported rather than left looking like a clean finish")
				assert.True(t, ops.Collectable(&op.Spec.OperationSpec, &status.OperationStatus),
					"a label nobody is coming back to clear must not pin the operation")
			}
		})
	}
}

// A Pending operation has not acquired the beacon yet, so its absence is "not created" rather than
// "lost" — the system-agent controller creates one once the cluster can take operations — and the
// operation waits rather than failing.
func TestOnChange_MissingBeaconWhilePendingWaits(t *testing.T) {
	t.Parallel()

	op := withClusterRef(newOp(), "test")
	op.Spec.TTL = -1
	op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
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
			t.Parallel()

			op := withClusterRef(newOp(), "gone")
			op.Spec.TTL = -1
			op.Status = opv1alpha1.ETCDSnapshotSaveStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
				Step:            opv1alpha1.ETCDSnapshotSaveStepSave,
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

// fakePlanSecrets serves machine-plan secrets to the collector, honoring the label selector the way
// the real cache does, and records the plans canceled through it into a log shared with
// fakeBeaconClient so their relative order is observable.
type fakePlanSecrets struct {
	corecontrollers.SecretClient

	items   []*corev1.Secret
	events  *[]string
	updates []*corev1.Secret
}

func (f *fakePlanSecrets) List(namespace string, opts metav1.ListOptions) (*corev1.SecretList, error) {
	selector, err := labels.Parse(opts.LabelSelector)
	if err != nil {
		return nil, err
	}

	var out corev1.SecretList
	for _, secret := range f.items {
		if secret.Namespace != namespace || !selector.Matches(labels.Set(secret.Labels)) {
			continue
		}
		out.Items = append(out.Items, *secret)
	}
	return &out, nil
}

func (f *fakePlanSecrets) Update(secret *corev1.Secret) (*corev1.Secret, error) {
	if f.events != nil {
		*f.events = append(*f.events, "cancel-plan/"+secret.Name)
	}
	f.updates = append(f.updates, secret.DeepCopy())
	return secret, nil
}

// A canceled operation has to stop the work it already handed to the agents, not just record that
// it was called off: a plan sitting in a machine-plan secret is the agent's to run, and releasing
// the beacon lets the next operation start on the same cluster. So the plans go first, and this
// asserts that order rather than just the two writes.
func TestHandleCanceled_CancelsDispatchedPlansBeforeReleasingBeacon(t *testing.T) {
	t.Parallel()

	op := newOp()
	s := newScope(op, newBeacon(testOwnerKey, true), defaultAdapter())

	var events []string
	secrets := &fakePlanSecrets{events: &events, items: []*corev1.Secret{
		withDispatchedPlan(t, newPlanSecret("node-a"), op),
	}}
	beacons := &fakeBeaconClient{beacon: s.beacon, events: &events}

	h := &handler{beacons: beacons, secrets: secrets, store: planapi.NewStore(secrets), dynamic: &fakeDynamic{}}

	status := opv1alpha1.ETCDSnapshotSaveStatus{}
	status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")

	got, err := h.handleCanceled(s, status)
	require.NoError(t, err)
	assert.False(t, got.TerminatedAt.IsZero(), "the terminal handling completed")
	assert.Equal(t, []string{"cancel-plan/node-a", "beacon-write"}, events,
		"the plans this operation dispatched must be canceled while it is still the authorized writer")
	require.Len(t, secrets.updates, 1)
	assert.Equal(t, "true", secrets.updates[0].Annotations[planapi.PlanCanceledAnnotation])
}

// withDispatchedPlan returns a copy of secret holding a plan this operation dispatched, i.e. one
// stamped with the operation environment the way the step reconcilers assign it.
func withDispatchedPlan(t *testing.T, secret *corev1.Secret, op *opv1alpha1.ETCDSnapshotSave) *corev1.Secret {
	t.Helper()

	nodePlan := &planapi.Plan{OneTimeInstructions: []planapi.OneTimeInstruction{{Name: "work", Command: "rke2"}}}
	data, err := json.Marshal(ops.WithOperationEnv(nodePlan, ops.OperationEnv(ControllerOwnerKey, op, opv1alpha1.ETCDSnapshotSaveStepRestart)))
	require.NoError(t, err)

	out := secret.DeepCopy()
	if out.Data == nil {
		out.Data = map[string][]byte{}
	}
	out.Data[planapi.PlanDataKey] = data
	return out
}
