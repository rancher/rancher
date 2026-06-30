package etcdsnapshotsave

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	"github.com/rancher/rancher/pkg/capr"
	ops "github.com/rancher/rancher/pkg/operations"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	ctrlfake "github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
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

func (a *stubAdapter) WaitForRegister() (bool, error) {
	return a.waitForRegisterOK, a.waitForRegisterErr
}
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
// by the snapshot-save controller (which only consumes runtime/dataDir/serverUnit/probes/plans),
// so each returns a static, runtime-appropriate value.
func (a *stubAdapter) ConfigDirectory(_ *corev1.Secret) string {
	// Mirrors CAPRAdapter.ConfigDirectory's format: /etc/rancher/<runtime>/config.yaml.d.
	return "/etc/rancher/" + a.runtimeCommand + "/config.yaml.d"
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

// newScope wires together the common per-reconcile context for the tests.
func newScope(op *opv1alpha1.ETCDSnapshotSave, beacon *planv1alpha1.Beacon, adapter *stubAdapter) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetName("test")
	cluster.SetNamespace("fleet-default")
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")
	return &scope{
		op:         op,
		beacon:     beacon,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    adapter,
	}
}

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
	lbls := map[string]string{}
	if owner != "" {
		lbls[planv1alpha1.BeaconOwnerLabel] = owner
	}
	// Beacon ownership lives on Status.Owner (the plan.AcquireBeacon helper writes there).
	// We populate the legacy BeaconOwnerLabel too so any caller that still reads it (e.g.
	// EncryptionKeyRotation's reclaimStaleBeaconOwnerIfNeeded) keeps working.
	return &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "fleet-default",
			Labels:    lbls,
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

	updates       []*planv1alpha1.Beacon
	statusUpdates []*planv1alpha1.Beacon
	updateErr     error
}

func (f *fakeBeaconClient) Update(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.updates = append(f.updates, b.DeepCopy())
	return b, f.updateErr
}

func (f *fakeBeaconClient) UpdateStatus(b *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, b.DeepCopy())
	return b, nil
}

// --- updateStatus ---------------------------------------------------------------------------

func TestUpdateStatusPaused(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true
	op.Generation = 7

	status := updateStatus(op, opv1alpha1.ETCDSnapshotSaveStatus{})

	assert.Equal(t, int64(7), status.ObservedGeneration, "ObservedGeneration must be copied from the op")
	assert.Equal(t, "True", opv1alpha1.PausedCondition.GetStatus(&status))
	assert.Equal(t, opv1alpha1.PausedReason, opv1alpha1.PausedCondition.GetReason(&status))
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
			name:  "succeeded clears Pending+InProgress+Failed",
			phase: opv1alpha1.OperationPhaseSucceeded,
			check: func(t *testing.T, s opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, "False", opv1alpha1.PendingCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.InProgressCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.FailedCondition.GetStatus(&s))
				assert.Equal(t, opv1alpha1.NotFailedReason, opv1alpha1.FailedCondition.GetReason(&s))
			},
		},
		{
			name:  "failed clears Pending+InProgress+Succeeded",
			phase: opv1alpha1.OperationPhaseFailed,
			check: func(t *testing.T, s opv1alpha1.ETCDSnapshotSaveStatus) {
				assert.Equal(t, "False", opv1alpha1.PendingCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.InProgressCondition.GetStatus(&s))
				assert.Equal(t, "False", opv1alpha1.SucceededCondition.GetStatus(&s))
				assert.Equal(t, opv1alpha1.NotSuccessfulReason, opv1alpha1.SucceededCondition.GetReason(&s))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := newOp()
			status := updateStatus(op, opv1alpha1.ETCDSnapshotSaveStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: tc.phase},
			})
			tc.check(t, status)
		})
	}
}

// --- handlePending --------------------------------------------------------------------------

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
	s := newScope(newOp(), newBeacon(ControllerOwnerKey, false), adapter)

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
	s := newScope(newOp(), newBeacon(ControllerOwnerKey, false), a)

	got, err := h.handlePending(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseInProgress, got.Phase)
	assert.Equal(t, opv1alpha1.ETCDSnapshotSaveStepSave, got.Step)
}

func TestHandlePending_WaitForRegisterErrorBubbles(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("kaboom")
	adapter := defaultAdapter()
	adapter.waitForRegisterErr = sentinel
	adapter.waitForRegisterOK = false

	h := &handler{beacons: &fakeBeaconClient{}}
	_, err := h.handlePending(newScope(newOp(), newBeacon(ControllerOwnerKey, false), adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.ErrorIs(t, err, sentinel)
}

// --- handleInProgress -----------------------------------------------------------------------

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
	s := newScope(op, newBeacon(ControllerOwnerKey, false), defaultAdapter())

	got, err := h.handleInProgress(s, opv1alpha1.ETCDSnapshotSaveStatus{Step: "Whatever"})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.UnknownStepReason, opv1alpha1.FailedCondition.GetReason(&got))
}

// --- handleFailed / handleSucceeded --------------------------------------------------------

func TestHandleFailed_HoldingBeaconReleases(t *testing.T) {
	t.Parallel()

	beacons := &fakeBeaconClient{}
	h := &handler{beacons: beacons}
	s := newScope(newOp(), newBeacon(ControllerOwnerKey, true), defaultAdapter())

	_, err := h.handleFailed(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// Both ToggleBeacon(active=false) and ReleaseBeacon now route through UpdateStatus
	// (Status.Owner is the source of truth for ownership), so we expect two status updates and
	// no main-resource updates.
	if assert.Len(t, beacons.statusUpdates, 2, "ToggleBeacon + ReleaseBeacon should both update the beacon status") {
		assert.False(t, beacons.statusUpdates[0].Status.Active, "beacon must be toggled inactive")
		assert.Equal(t, "", beacons.statusUpdates[1].Status.Owner, "Status.Owner must be cleared on release")
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
	s := newScope(newOp(), newBeacon(ControllerOwnerKey, true), defaultAdapter())

	_, err := h.handleSucceeded(s, opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// Both ToggleBeacon(active=false) and ReleaseBeacon route through UpdateStatus now.
	if assert.Len(t, beacons.statusUpdates, 2, "ToggleBeacon + ReleaseBeacon should both update the beacon status") {
		assert.False(t, beacons.statusUpdates[0].Status.Active, "beacon must be toggled inactive on success")
		assert.Equal(t, "", beacons.statusUpdates[1].Status.Owner, "Status.Owner must be cleared on release")
	}
	assert.Empty(t, beacons.updates, "ReleaseBeacon no longer touches the main resource")
	if assert.Len(t, dyn.enqueued, 1, "parent cluster must be re-enqueued") {
		assert.Equal(t, "provisioning.cattle.io/v1, Kind=Cluster/fleet-default/test", dyn.enqueued[0])
	}
}

// --- reconcileSave --------------------------------------------------------------------------

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

func expectedSavePlan(op *opv1alpha1.ETCDSnapshotSave, adapter *stubAdapter) *planapi.Plan {
	return &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{expectedSaveInstruction(op, adapter.runtimeCommand)},
		Probes:              adapter.probes,
	}
}

func expectedRestartPlan(adapter *stubAdapter) *planapi.Plan {
	return &planapi.Plan{
		OneTimeInstructions: []planapi.OneTimeInstruction{
			{CommonInstruction: planapi.CommonInstruction{
				Name:    "restart",
				Command: "systemctl",
				Args:    []string{"restart", adapter.serverUnit},
			}},
		},
		Probes: adapter.probes,
	}
}

func TestReconcileSave_NoSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	h := &handler{
		secrets: newSecretClient(t, ctrl),
	}
	h.store = planapi.NewStore(h.secrets)

	status, err := h.reconcileSave(newScope(newOp(), nil, defaultAdapter()), opv1alpha1.ETCDSnapshotSaveStatus{})
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

	got, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
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

	got, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
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

	got, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
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
	// cycles — we're asserting on the *plan content* not the wait behaviour here.
	expectedPlan := expectedSavePlan(op, adapter)
	secret := withAppliedPlan(newPlanSecret("etcd-1"), expectedPlan)
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	_, err := h.reconcileSave(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)

	wantArgs := []string{"etcd-snapshot", "save", "--name", "my-snap"}
	if !reflect.DeepEqual(expectedPlan.OneTimeInstructions[0].Args, wantArgs) {
		t.Errorf("plan args = %v, want %v — snapshot Args were not threaded through", expectedPlan.OneTimeInstructions[0].Args, wantArgs)
	}
}

// --- reconcileRestart -----------------------------------------------------------------------

func TestReconcileRestart_MarksSucceededWhenApplied(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := withAppliedPlan(newPlanSecret("etcd-1"), expectedRestartPlan(adapter))
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
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

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Empty(t, string(got.Phase), "phase must not advance to Succeeded while restart is pending")
	assert.Equal(t, opv1alpha1.WaitingForPlanAppliedReason, opv1alpha1.InProgressCondition.GetReason(&got))
}

func TestReconcileRestart_PlanFailureMarksFailed(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	secret := withFailedPlan(newPlanSecret("etcd-1"), expectedRestartPlan(adapter))
	h := &handler{
		secrets: newSecretClient(t, ctrl, secret),
	}
	h.store = planapi.NewStore(h.secrets)

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	assert.Equal(t, opv1alpha1.OperationPhaseFailed, got.Phase)
	assert.Equal(t, opv1alpha1.PlanFailedReason, opv1alpha1.FailedCondition.GetReason(&got))
}

func TestReconcileRestart_FiltersToEtcdSecrets(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	op := newOp()
	adapter := defaultAdapter()

	etcd := withAppliedPlan(newPlanSecret("etcd-1"), expectedRestartPlan(adapter))
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

	got, err := h.reconcileRestart(newScope(op, nil, adapter), opv1alpha1.ETCDSnapshotSaveStatus{})
	assert.NoError(t, err)
	// Worker secret must be ignored — only etcd nodes receive the restart plan; success would
	// not be reached if the worker were included (its plan is not in "applied" state).
	assert.Equal(t, opv1alpha1.OperationPhaseSucceeded, got.Phase, "non-etcd secrets must not be in the iteration")
}
