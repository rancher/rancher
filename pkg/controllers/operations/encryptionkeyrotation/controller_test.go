package encryptionkeyrotation

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	"github.com/rancher/rancher/pkg/wrangler"
	"github.com/rancher/wrangler/v3/pkg/condition"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type stubAdapter struct {
	waitForRegisterOK  bool
	waitForRegisterErr error
	pauseCalls         []bool
	pauseErr           error
}

func (a *stubAdapter) BeaconRef() (string, string) { return "test-namespace", "test-cluster" }

func (a *stubAdapter) EtcdSnapshotNamespace() string { return "test-namespace" }

func (a *stubAdapter) ClusterObject() (*unstructured.Unstructured, error) {
	return &unstructured.Unstructured{}, nil
}

// The restore-target and install methods complete the ops.Adapter contract; only the etcd snapshot
// restore controller uses them.
func (a *stubAdapter) RestoreTarget(_ string) (*unstructured.Unstructured, error) { return nil, nil }
func (a *stubAdapter) UpdateRestoreTarget(_ *unstructured.Unstructured) error     { return nil }
func (a *stubAdapter) WaitForRestoreTarget() (bool, error)                        { return true, nil }
func (a *stubAdapter) InstallInstruction(_ *corev1.Secret, _ string) (plan.OneTimeInstruction, bool) {
	return plan.OneTimeInstruction{}, false
}

func (a *stubAdapter) WaitForRegister() (bool, error) {
	return a.waitForRegisterOK, a.waitForRegisterErr
}

func (a *stubAdapter) RuntimeCommand() string {
	return "rke2"
}

func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) (string, error) {
	return "/var/lib/rancher/rke2", nil
}
func (a *stubAdapter) DistroManifestPaths(_ string) ops.ManifestPaths {
	return ops.ManifestPaths{}
}

func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	return "/var/lib/rancher/capr"
}
func (a *stubAdapter) ServerUnit() string {
	return "rke2-server"
}

func (a *stubAdapter) RuntimeService(_ *corev1.Secret) string {
	return "rke2-server"
}

func (a *stubAdapter) DistroServices(secret *corev1.Secret) []string {
	return ops.DistroServices(a.RuntimeCommand(), secret)
}

func (a *stubAdapter) RenderProbes(_ *corev1.Secret, _ bool) (map[string]rkeplan.Probe, error) {
	return map[string]rkeplan.Probe{}, nil
}

func (a *stubAdapter) KubectlPath(_ *corev1.Secret) (string, error) {
	return "/var/lib/rancher/rke2/bin/kubectl", nil
}

func (a *stubAdapter) KubeconfigPath(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/rke2.yaml"
}

func (a *stubAdapter) FindOrElectLeader(_ string, _ ops.Filter) (*corev1.Secret, error) {
	return nil, nil
}

func (a *stubAdapter) PauseCluster(paused bool) error {
	if a.pauseErr != nil {
		return a.pauseErr
	}
	a.pauseCalls = append(a.pauseCalls, paused)
	return nil
}

// The six methods below complete the ops.Adapter contract for the stub. None of them are
// exercised by the encryption-key-rotation controller (which only consumes runtime/dataDir/
// serverUnit/probes/pause/plans), so each returns a static RKE2-shaped value matching what
// CAPRAdapter would produce for an rke2 cluster.
func (a *stubAdapter) ConfigFile(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/config.yaml"
}
func (a *stubAdapter) ConfigDirectory(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/config.yaml.d"
}
func (a *stubAdapter) ComponentTLSSettings(_ *corev1.Secret, _ string) (ops.ComponentTLSSettings, error) {
	return ops.ComponentTLSSettings{}, nil
}
func (a *stubAdapter) GetServerURL(_ *corev1.Secret) string      { return "" }
func (a *stubAdapter) GetSupervisorPort(_ *corev1.Secret) string { return "9345" }
func (a *stubAdapter) LoopbackAddress(_ *corev1.Secret) string   { return "127.0.0.1" }
func (a *stubAdapter) ToS3ArgsEnvAndFiles(_ *corev1.Secret) ([]string, []string, []plan.File) {
	return nil, nil, nil
}

type enqueueCall struct {
	gvk       schema.GroupVersionKind
	namespace string
	name      string
}

type fakeDynamic struct {
	getObj       runtime.Object
	getErr       error
	enqueueErr   error
	enqueueCalls []enqueueCall
}

// Get honors the requested name, so a test can point an operation at a cluster which is not there:
// what the reconcile sees once a cluster has been deleted underneath an operation. Without that a
// missing-cluster test resolves a scope as usual and asserts against the wrong code path.
func (d *fakeDynamic) Get(_ schema.GroupVersionKind, _, name string) (runtime.Object, error) {
	if d.getErr != nil {
		return nil, d.getErr
	}

	obj, ok := d.getObj.(metav1.Object)
	if !ok || obj.GetName() != name {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, name)
	}

	return d.getObj, nil
}

func (d *fakeDynamic) Enqueue(gvk schema.GroupVersionKind, namespace, name string) error {
	d.enqueueCalls = append(d.enqueueCalls, enqueueCall{
		gvk:       gvk,
		namespace: namespace,
		name:      name,
	})
	return d.enqueueErr
}

func newOp() *opv1alpha1.EncryptionKeyRotation {
	return &opv1alpha1.EncryptionKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ekr-1",
			Namespace: "fleet-default",
			UID:       types.UID("ekr-uid"),
		},
	}
}

func newBeacon(owner string, active bool) *planv1alpha1.Beacon {
	// Beacon ownership lives on Status.Owner; we keep the legacy BeaconOwnerLabel populated so
	// reclaimStaleBeaconOwnerIfNeeded (which still reads the label) sees a consistent owner.
	return &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fleet-default",
			Namespace: "fleet-default",
		},
		Status: planv1alpha1.BeaconStatus{
			Active: active,
			Owner:  owner,
		},
	}
}

func newScope(op *opv1alpha1.EncryptionKeyRotation, beacon *planv1alpha1.Beacon, adapter ops.Adapter) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test")
	return &scope{
		// The ownerKey the real controller computes in resolveScope, so beacon fixtures created
		// with ops.BeaconOwnerKey(OperationKind, op) are recognized as ours by the ownership and delegate checks.
		ownerKey:   ops.BeaconOwnerKey(OperationKind, op),
		op:         op,
		beacon:     beacon,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    adapter,
	}
}

type fakeEncryptionKeyRotationController struct {
	operationcontrollers.EncryptionKeyRotationController
	getFn        func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error)
	enqueueCalls int
	deleteCalls  int
	// updates records the objects passed to Update — the finalizer is the only thing the handler
	// writes outside of status, so each entry is a finalizer add or removal.
	updates []*opv1alpha1.EncryptionKeyRotation
}

func (f *fakeEncryptionKeyRotationController) Update(op *opv1alpha1.EncryptionKeyRotation) (*opv1alpha1.EncryptionKeyRotation, error) {
	f.updates = append(f.updates, op.DeepCopy())
	return op, nil
}

func (f *fakeEncryptionKeyRotationController) Get(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
	if f.getFn == nil {
		return nil, nil
	}
	return f.getFn(namespace, name, opts)
}

func (f *fakeEncryptionKeyRotationController) EnqueueAfter(_, _ string, _ time.Duration) {
	f.enqueueCalls++
}

func (f *fakeEncryptionKeyRotationController) Delete(_, _ string, _ *metav1.DeleteOptions) error {
	f.deleteCalls++
	return nil
}

type fakeBeaconClient struct {
	plancontrollers.BeaconClient
	// beacon is what Get serves, and is kept in step with Update/UpdateStatus so a handler driven
	// over several reconciles observes its own beacon writes. A nil beacon makes Get report
	// NotFound.
	beacon          *planv1alpha1.Beacon
	updates         []*planv1alpha1.Beacon
	statusUpdates   []*planv1alpha1.Beacon
	updateErr       error
	updateStatusErr error
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
func (f *fakeBeaconClient) Update(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	updated := beacon.DeepCopy()
	if f.beacon != nil {
		updated.Status = f.beacon.Status
	}
	f.updates = append(f.updates, updated.DeepCopy())
	if f.updateErr == nil {
		f.beacon = updated.DeepCopy()
	}
	return updated, f.updateErr
}

func (f *fakeBeaconClient) UpdateStatus(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, beacon.DeepCopy())
	if f.updateStatusErr == nil {
		f.beacon = beacon.DeepCopy()
	}
	return beacon, f.updateStatusErr
}

func newPeriodicStatusSecret(secretName, stdout string) *corev1.Secret {
	periodicOutput := map[string]plan.PeriodicInstructionOutput{
		statusPeriodicName: {
			Name:   statusPeriodicName,
			Stdout: []byte(stdout),
		},
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: "fleet-default",
		},
		Data: map[string][]byte{
			"applied-periodic-output": mustGzipJSON(periodicOutput),
		},
	}
}

func mustGzipJSON(v any) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	if _, err := writer.Write(raw); err != nil {
		panic(err)
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}

	return buffer.Bytes()
}

func TestConvergenceWaitMessage(t *testing.T) {
	secretName := "leader-node"
	tests := []struct {
		name             string
		stdout           string
		requireHashMatch bool
		wantWait         bool
		wantErr          bool
	}{
		{
			name:             "returns wait message for non-final stage",
			stdout:           "Current Rotation Stage: start\nServer Encryption Hashes: All hashes match",
			requireHashMatch: false,
			wantWait:         true,
		},
		{
			name:             "returns success at reencrypt finished without hash requirement",
			stdout:           "Current Rotation Stage: reencrypt_finished",
			requireHashMatch: false,
		},
		{
			name:             "returns success at reencrypt finished with hash match",
			stdout:           "Current Rotation Stage: reencrypt_finished\nServer Encryption Hashes: All hashes match",
			requireHashMatch: true,
		},
		{
			name:             "returns wait at reencrypt finished while hashes differ",
			stdout:           "Current Rotation Stage: reencrypt_finished\nServer Encryption Hashes: hash mismatch",
			requireHashMatch: true,
			wantWait:         true,
		},
		{
			name:             "returns error for malformed status output",
			stdout:           "Server Encryption Hashes: All hashes match",
			requireHashMatch: false,
			wantErr:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret := newPeriodicStatusSecret(secretName, tt.stdout)
			waitMsg, err := convergenceWaitMessage(secret, tt.requireHashMatch)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantWait && waitMsg == "" {
				t.Fatalf("expected wait message, got empty")
			}
			if !tt.wantWait && waitMsg != "" {
				t.Fatalf("expected no wait message, got: %s", waitMsg)
			}
		})
	}
}

func TestReadRotateKeysResult(t *testing.T) {
	tests := []struct {
		name          string
		appliedOutput map[string][]byte
		wantExitCode  int
		wantOutput    string
		wantNotYet    bool
		wantErr       bool
	}{
		{
			name: "parses valid exit code line",
			appliedOutput: map[string][]byte{
				rotateKeysInstructionName: []byte("rotate output\n" + exitCodePrefix + "7\n"),
			},
			wantExitCode: 7,
			wantOutput:   "rotate output\n" + exitCodePrefix + "7\n",
		},
		{
			name:          "returns not yet when key missing",
			appliedOutput: map[string][]byte{},
			wantNotYet:    true,
			wantErr:       true,
		},
		{
			name: "returns not yet when exit code line missing",
			appliedOutput: map[string][]byte{
				rotateKeysInstructionName: []byte("rotate output without code"),
			},
			wantNotYet: true,
			wantErr:    true,
		},
		{
			name: "returns parse error for corrupt exit code",
			appliedOutput: map[string][]byte{
				rotateKeysInstructionName: []byte("rotate output\n" + exitCodePrefix + "NaN\n"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := readRotateKeysResult(tt.appliedOutput)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantNotYet && !errors.Is(err, errRotateKeysOutputNotYet) {
					t.Fatalf("expected errRotateKeysOutputNotYet, got %v", err)
				}
				if !tt.wantNotYet && errors.Is(err, errRotateKeysOutputNotYet) {
					t.Fatalf("expected non-sentinel error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.exitCode != tt.wantExitCode {
				t.Fatalf("expected exit code %d, got %d", tt.wantExitCode, result.exitCode)
			}
			if result.output != tt.wantOutput {
				t.Fatalf("expected output %q, got %q", tt.wantOutput, result.output)
			}
		})
	}
}

func TestStatusFromOutput(t *testing.T) {
	tests := []struct {
		name              string
		output            string
		wantStage         string
		wantHashesMatch   bool
		wantHashesPresent bool
		wantTimeoutErr    bool
		wantErr           bool
	}{
		{
			name:      "parses start stage",
			output:    "Current Rotation Stage: start",
			wantStage: "start",
		},
		{
			name:              "parses reencrypt finished with matching hashes",
			output:            "Current Rotation Stage: reencrypt_finished\nServer Encryption Hashes: All hashes match",
			wantStage:         "reencrypt_finished",
			wantHashesMatch:   true,
			wantHashesPresent: true,
		},
		{
			name:              "parses reencrypt finished with non-matching hashes",
			output:            "Current Rotation Stage: reencrypt_finished\nServer Encryption Hashes: hash does not match",
			wantStage:         "reencrypt_finished",
			wantHashesPresent: true,
		},
		{
			name:           "returns timeout error for known timeout output",
			output:         "see server log for details: Get https://127.0.0.1:9345/encrypt/status: context deadline exceeded",
			wantTimeoutErr: true,
			wantErr:        true,
		},
		{
			name:    "returns error when stage line is missing",
			output:  "Server Encryption Hashes: All hashes match",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := statusFromOutput(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.wantTimeoutErr && !errors.Is(err, errStatusTimeout) {
					t.Fatalf("expected errStatusTimeout, got %v", err)
				}
				if !tt.wantTimeoutErr && errors.Is(err, errStatusTimeout) {
					t.Fatalf("expected non-timeout error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.stage != tt.wantStage {
				t.Fatalf("expected stage %q, got %q", tt.wantStage, status.stage)
			}
			if status.hashesMatch != tt.wantHashesMatch {
				t.Fatalf("expected hashesMatch %t, got %t", tt.wantHashesMatch, status.hashesMatch)
			}
			if status.hashesPresent != tt.wantHashesPresent {
				t.Fatalf("expected hashesPresent %t, got %t", tt.wantHashesPresent, status.hashesPresent)
			}
		})
	}
}

func TestUpdateStatusByPhase(t *testing.T) {
	tests := []struct {
		name       string
		phase      opv1alpha1.OperationPhase
		terminated bool
		check      func(t *testing.T, status opv1alpha1.EncryptionKeyRotationStatus)
	}{
		{
			name:  "pending sets Pending=true",
			phase: opv1alpha1.OperationPhasePending,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.PendingCondition.GetStatus(&s)) != "True" {
					t.Fatalf("expected PendingCondition=True")
				}
			},
		},
		{
			name:  "in-progress clears pending with in-progress reason",
			phase: opv1alpha1.OperationPhaseInProgress,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.PendingCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected PendingCondition=False")
				}
				if opv1alpha1.PendingCondition.GetReason(&s) != opv1alpha1.InProgressReason {
					t.Fatalf("expected PendingCondition reason %q, got %q", opv1alpha1.InProgressReason, opv1alpha1.PendingCondition.GetReason(&s))
				}
			},
		},
		{
			// The outcome is asserted as soon as the phase is reached; only Finalized waits for the
			// controller to be done with the operation (hook satisfied, cluster unpaused, beacon
			// released).
			name:  "succeeded asserts the outcome and finalizes",
			phase: opv1alpha1.OperationPhaseSucceeded,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.SucceededCondition.GetStatus(&s)) != "True" {
					t.Fatalf("expected SucceededCondition=True once the phase is reached")
				}
				if string(opv1alpha1.FailedCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected FailedCondition=False")
				}
				if string(opv1alpha1.InProgressCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected InProgressCondition=False")
				}
				if opv1alpha1.InProgressCondition.GetReason(&s) != opv1alpha1.FinalizingReason {
					t.Fatalf("expected InProgressCondition reason %q, got %q", opv1alpha1.FinalizingReason, opv1alpha1.InProgressCondition.GetReason(&s))
				}
				if string(opv1alpha1.FinalizedCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected FinalizedCondition=False while finalizing")
				}
				if opv1alpha1.FinalizedCondition.GetReason(&s) != opv1alpha1.FinalizingReason {
					t.Fatalf("expected FinalizedCondition reason %q, got %q", opv1alpha1.FinalizingReason, opv1alpha1.FinalizedCondition.GetReason(&s))
				}
			},
		},
		{
			name:  "failed asserts the outcome and finalizes",
			phase: opv1alpha1.OperationPhaseFailed,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.FailedCondition.GetStatus(&s)) != "True" {
					t.Fatalf("expected FailedCondition=True once the phase is reached")
				}
				if string(opv1alpha1.SucceededCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected SucceededCondition=False")
				}
				if string(opv1alpha1.FinalizedCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected FinalizedCondition=False while finalizing")
				}
			},
		},
		{
			name:  "terminated asserts the outcome and finalizes",
			phase: opv1alpha1.OperationPhaseSucceeded,
			// terminated is applied by the runner below.
			terminated: true,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.SucceededCondition.GetStatus(&s)) != "True" {
					t.Fatalf("expected SucceededCondition=True once terminated")
				}
				if string(opv1alpha1.FinalizedCondition.GetStatus(&s)) != "True" {
					t.Fatalf("expected FinalizedCondition=True once terminated")
				}
				if string(opv1alpha1.FailedCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected FailedCondition=False")
				}
				if opv1alpha1.FailedCondition.GetReason(&s) != opv1alpha1.NotFailedReason {
					t.Fatalf("expected FailedCondition reason %q, got %q", opv1alpha1.NotFailedReason, opv1alpha1.FailedCondition.GetReason(&s))
				}
				if string(opv1alpha1.PendingCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected PendingCondition=False")
				}
				if string(opv1alpha1.InProgressCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected InProgressCondition=False")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := newOp()
			op.Generation = 42

			initial := opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: tt.phase},
			}
			if tt.terminated {
				initial.SetTerminated()
			}

			status := updateStatus(op, initial)
			if status.ObservedGeneration != 42 {
				t.Fatalf("expected ObservedGeneration=42, got %d", status.ObservedGeneration)
			}
			tt.check(t, status)
		})
	}
}

func TestHandleFailed_HoldingBeaconReleasesAndLeavesClusterPaused(t *testing.T) {
	op := newOp()
	adapter := &stubAdapter{waitForRegisterOK: true}
	beacons := &fakeBeaconClient{}

	h := &handler{beacons: beacons}
	s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter)

	_, err := h.handleFailed(s, opv1alpha1.EncryptionKeyRotationStatus{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A rotation which failed left the cluster mid-rotation, so it stays paused for an
	// administrator to resolve. Only reconcileRestart, having restarted every node, unpauses.
	if len(adapter.pauseCalls) != 0 {
		t.Fatalf("terminal handling must not unpause the cluster, got %+v", adapter.pauseCalls)
	}
	// ReleaseBeacon writes to UpdateStatus (Status.Owner is the source of truth for beacon
	// ownership); the legacy main-resource Update is no longer used.
	if len(beacons.statusUpdates) != 1 {
		t.Fatalf("expected one beacon status update (ReleaseBeacon), got %d", len(beacons.statusUpdates))
	}
	if beacons.statusUpdates[0].Status.Owner != "" {
		t.Fatalf("expected Status.Owner to be cleared on release, got %q", beacons.statusUpdates[0].Status.Owner)
	}
	if len(beacons.updates) != 0 {
		t.Fatalf("expected no main-resource updates from ReleaseBeacon, got %d", len(beacons.updates))
	}
}

func TestHandleSucceeded_HoldingBeaconTogglesReleasesAndEnqueues(t *testing.T) {
	op := newOp()
	adapter := &stubAdapter{waitForRegisterOK: true}
	beacons := &fakeBeaconClient{}
	dynamic := &fakeDynamic{}

	h := &handler{
		beacons: beacons,
		dynamic: dynamic,
	}
	s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter)

	_, err := h.handleSucceeded(s, opv1alpha1.EncryptionKeyRotationStatus{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// reconcileRestart has already unpaused the cluster by the time an operation succeeds, so
	// terminal handling has nothing to undo.
	if len(adapter.pauseCalls) != 0 {
		t.Fatalf("terminal handling must not unpause the cluster, got %+v", adapter.pauseCalls)
	}
	// ReleaseBeacon on the owner path clears Active + Owner + Delegates in a single
	// UpdateStatus call — no separate ToggleBeacon is needed.
	if len(beacons.statusUpdates) != 1 {
		t.Fatalf("expected one beacon status update (release), got %d", len(beacons.statusUpdates))
	}
	if beacons.statusUpdates[0].Status.Active {
		t.Fatalf("expected beacon to be toggled inactive on release")
	}
	if beacons.statusUpdates[0].Status.Owner != "" {
		t.Fatalf("expected Status.Owner to be cleared on release, got %q", beacons.statusUpdates[0].Status.Owner)
	}
	if len(beacons.updates) != 0 {
		t.Fatalf("expected no main-resource updates from ReleaseBeacon, got %d", len(beacons.updates))
	}

	if len(dynamic.enqueueCalls) != 1 {
		t.Fatalf("expected one cluster enqueue, got %d", len(dynamic.enqueueCalls))
	}
	expectedGVK := schema.FromAPIVersionAndKind("provisioning.cattle.io/v1", "Cluster")
	if dynamic.enqueueCalls[0].gvk != expectedGVK || dynamic.enqueueCalls[0].namespace != "fleet-default" || dynamic.enqueueCalls[0].name != "test" {
		t.Fatalf("unexpected enqueue call: %#v", dynamic.enqueueCalls[0])
	}
}

func TestHandleSucceeded_NotHoldingIsANoOp(t *testing.T) {
	op := newOp()
	adapter := &stubAdapter{waitForRegisterOK: true}
	beacons := &fakeBeaconClient{}
	dynamic := &fakeDynamic{}

	h := &handler{
		beacons: beacons,
		dynamic: dynamic,
	}
	s := newScope(op, newBeacon("other-controller", true), adapter)

	_, err := h.handleSucceeded(s, opv1alpha1.EncryptionKeyRotationStatus{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(adapter.pauseCalls) != 0 {
		t.Fatalf("terminal handling must not unpause the cluster, got %+v", adapter.pauseCalls)
	}
	if len(beacons.statusUpdates) != 0 {
		t.Fatalf("expected no beacon status updates, got %d", len(beacons.statusUpdates))
	}
	if len(beacons.updates) != 0 {
		t.Fatalf("expected no beacon release updates, got %d", len(beacons.updates))
	}
	if len(dynamic.enqueueCalls) != 0 {
		t.Fatalf("expected no enqueue calls, got %d", len(dynamic.enqueueCalls))
	}
}

func TestHandleInProgress_BeaconLost(t *testing.T) {
	op := newOp()
	op.Status.Step = opv1alpha1.EncryptionKeyRotationStepRotate

	h := &handler{beacons: &fakeBeaconClient{}}
	s := newScope(op, newBeacon("other-controller", true), &stubAdapter{waitForRegisterOK: true})

	got, err := h.handleInProgress(s, opv1alpha1.EncryptionKeyRotationStatus{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Phase != opv1alpha1.OperationPhaseFailed {
		t.Fatalf("expected phase failed, got %q", got.Phase)
	}
	if opv1alpha1.FailedCondition.GetReason(&got) != opv1alpha1.BeaconLostReason {
		t.Fatalf("expected failed reason %q, got %q", opv1alpha1.BeaconLostReason, opv1alpha1.FailedCondition.GetReason(&got))
	}
}

func TestHandleInProgress_UnknownStep(t *testing.T) {
	op := newOp()
	op.Status.Step = "mystery-step"
	beacons := &fakeBeaconClient{}
	h := &handler{beacons: beacons}

	s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), false), nil)
	got, err := h.handleInProgress(s, opv1alpha1.EncryptionKeyRotationStatus{Step: "mystery-step"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Phase != opv1alpha1.OperationPhaseFailed {
		t.Fatalf("expected phase failed, got %q", got.Phase)
	}
	if opv1alpha1.FailedCondition.GetReason(&got) != opv1alpha1.UnknownStepReason {
		t.Fatalf("expected failed reason %q, got %q", opv1alpha1.UnknownStepReason, opv1alpha1.FailedCondition.GetReason(&got))
	}
	if len(beacons.statusUpdates) != 1 {
		t.Fatalf("expected one beacon status update before step handling, got %d", len(beacons.statusUpdates))
	}
	if !beacons.statusUpdates[0].Status.Active {
		t.Fatalf("expected beacon to be toggled active while operation is in progress")
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

			initialStatus := opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase: opv1alpha1.OperationPhaseInProgress,
				},
				Step: opv1alpha1.EncryptionKeyRotationStepRotate,
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

func TestOnChange_Paused(t *testing.T) {
	t.Parallel()

	op := newOp()
	op.Spec.Paused = true
	op.Generation = 7

	initialStatus := opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase: opv1alpha1.OperationPhaseInProgress,
		},
		Step: opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	op.Status = initialStatus

	h := &handler{}
	status, err := h.OnChange(op, op.Status)

	if err != nil {
		t.Fatalf("OnChange returned error: %v", err)
	}

	// Verify PausedCondition is set to True
	if got := opv1alpha1.PausedCondition.GetStatus(&status); got != "True" {
		t.Errorf("PausedCondition status = %q, want %q", got, "True")
	}
	if got := opv1alpha1.PausedCondition.GetReason(&status); got != opv1alpha1.PausedReason {
		t.Errorf("PausedCondition reason = %q, want %q", got, opv1alpha1.PausedReason)
	}
	if got := opv1alpha1.PausedCondition.GetMessage(&status); got != "Operation is paused" {
		t.Errorf("PausedCondition message = %q, want %q", got, "Operation is paused")
	}

	// Verify ObservedGeneration is updated
	if status.ObservedGeneration != int64(7) {
		t.Errorf("ObservedGeneration = %d, want 7", status.ObservedGeneration)
	}

	// Verify phase and step are preserved
	if status.Phase != initialStatus.Phase {
		t.Errorf("Phase = %q, want %q (unchanged)", status.Phase, initialStatus.Phase)
	}
	if status.Step != initialStatus.Step {
		t.Errorf("Step = %q, want %q (unchanged)", status.Step, initialStatus.Step)
	}
}

func TestOnChange_StablePausedOperation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		phase       opv1alpha1.OperationPhase
		step        opv1alpha1.EncryptionKeyRotationStep
		ttl         int64
		lastUpdated metav1.Time
	}{
		{
			name:        "stable in-progress paused operation",
			phase:       opv1alpha1.OperationPhaseInProgress,
			step:        opv1alpha1.EncryptionKeyRotationStepRotate,
			ttl:         300,
			lastUpdated: metav1.Now(),
		},
		{
			name:        "stable terminal expired paused operation",
			phase:       opv1alpha1.OperationPhaseSucceeded,
			step:        opv1alpha1.EncryptionKeyRotationStepRotate,
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

			initialStatus := opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{
					Phase:       tc.phase,
					LastUpdated: tc.lastUpdated,
				},
				Step: tc.step,
			}

			// Pre-compute the expected status with paused condition
			currentStatus := updateStatus(op, initialStatus)
			op.Status = currentStatus

			controller := &fakeEncryptionKeyRotationController{}
			h := &handler{
				encryptionkeyrotations: controller,
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

// --- terminal handling -----------------------------------------------------------------------

// ekrClusterGVK is the cluster kind the OnChange tests resolve through, registered with the ops
// adapter factory below so a reconcile can be driven end to end — cluster lookup, adapter, beacon
// lookup, phase dispatch — without standing up a real provisioning/CAPI cluster. A dedicated kind
// keeps the registration from shadowing the adapter of a kind that ships with Rancher.
var ekrClusterGVK = schema.GroupVersionKind{Group: "test.cattle.io", Version: "v1", Kind: "TestEKRCluster"}

func init() {
	ops.RegisterAdapter(ekrClusterGVK, func(_ *wrangler.CAPIContext, _ *unstructured.Unstructured) (ops.Adapter, error) {
		return &stubAdapter{}, nil
	})
}

// newOnChangeOp is newOp with the cluster reference the OnChange tests resolve, and a generation so
// ObservedGeneration is meaningful.
func newOnChangeOp() *opv1alpha1.EncryptionKeyRotation {
	op := newOp()
	op.Generation = 1
	op.Spec.ClusterRef = &corev1.ObjectReference{
		APIVersion: ekrClusterGVK.GroupVersion().String(),
		Kind:       ekrClusterGVK.Kind,
		Namespace:  "fleet-default",
		Name:       "test",
	}
	return op
}

// newDeletingOp returns an operation which has been deleted and still carries our finalizer — the
// state the API server leaves an in-flight operation in until the controller releases it.
func newDeletingOp() *opv1alpha1.EncryptionKeyRotation {
	op := newOnChangeOp()
	op.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	op.Finalizers = []string{Finalizer}
	return op
}

// newOnChangeHandler wires a handler for the full OnChange path: the cluster referenced by
// newOnChangeOp resolves through the dynamic resolver, and the beacon lookup serves (and mutates)
// the given beacon. A nil beacon makes the lookup report NotFound.
func newOnChangeHandler(beacon *planv1alpha1.Beacon) (*handler, *fakeEncryptionKeyRotationController, *fakeBeaconClient, *stubAdapter) {
	controller := &fakeEncryptionKeyRotationController{}
	beacons := &fakeBeaconClient{beacon: beacon}
	adapter := &stubAdapter{}

	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(ekrClusterGVK)
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test")

	h := &handler{
		encryptionkeyrotations: controller,
		beacons:                beacons,
		dynamic:                &fakeDynamic{getObj: cluster},
	}

	return h, controller, beacons, adapter
}

// terminalHandlers enumerates the three terminal phase handlers along with the condition each one
// reports through and the lifecycle-hook prefix that delegates it, so the tests below can assert
// the shared termination-recording contract once for all of them.
var terminalHandlers = map[string]struct {
	handle func(*handler, *scope, opv1alpha1.EncryptionKeyRotationStatus) (opv1alpha1.EncryptionKeyRotationStatus, error)
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
	for name, tc := range terminalHandlers {
		t.Run(name, func(t *testing.T) {
			op := newOp()
			beacons := &fakeBeaconClient{}
			adapter := &stubAdapter{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter)

			got, err := tc.handle(h, s, opv1alpha1.EncryptionKeyRotationStatus{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.TerminatedAt.IsZero() {
				t.Fatal("terminal handling completed, so it must be recorded on the status")
			}
			// Unpausing is reconcileRestart's job, not terminal handling's: only a rotation which
			// restarted every node has left the cluster fit to hand back to the planner.
			if len(adapter.pauseCalls) != 0 {
				t.Fatalf("terminal handling must not unpause the cluster, got %v", adapter.pauseCalls)
			}
			if len(beacons.statusUpdates) != 1 {
				t.Fatalf("expected one beacon status update (release), got %d", len(beacons.statusUpdates))
			}
			if beacons.statusUpdates[0].Status.Owner != "" {
				t.Fatalf("expected beacon owner cleared, got %q", beacons.statusUpdates[0].Status.Owner)
			}
		})
	}
}

// TestHandleTerminal_WithoutBeaconClaimLeavesItUntouched covers every outcome an operation can
// reach without holding the beacon — Failed after losing it, Aborted after being overtaken,
// Canceled by whoever wanted it next. In all three the operation still finishes, and the beacon
// (now someone else's) is left exactly as it is. The cluster stays paused: a rotation which did not
// restart every node has left it mid-rotation, and unpausing is reconcileRestart's job alone.
// Succeeded is excluded: it cannot be reached without holding the beacon throughout.
func TestHandleTerminal_WithoutBeaconClaimLeavesItUntouched(t *testing.T) {
	for name, tc := range terminalHandlers {
		if name == "succeeded" {
			continue
		}

		t.Run(name, func(t *testing.T) {
			op := newOp()
			op.Labels = map[string]string{tc.hook + "cleanup": "delegate-a"}

			beacons := &fakeBeaconClient{}
			adapter := &stubAdapter{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(op, newBeacon("another-controller", true), adapter)

			got, err := tc.handle(h, s, opv1alpha1.EncryptionKeyRotationStatus{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.TerminatedAt.IsZero() {
				t.Fatal("with no beacon to release, terminal handling is trivially complete")
			}
			if len(beacons.statusUpdates) != 0 || len(beacons.updates) != 0 {
				t.Fatalf("a beacon held by another controller must not be modified, got %d status updates",
					len(beacons.statusUpdates))
			}
			if len(adapter.pauseCalls) != 0 {
				t.Fatalf("terminal handling must not unpause the cluster, got %v", adapter.pauseCalls)
			}
		})
	}
}

// TestHandleTerminal_DelegatedDefersTermination is the counterpart: while a terminal phase hook is
// still delegated the handler has not finished, the beacon is still held on the operation's behalf,
// the cluster stays paused, and nothing may be recorded — that marker is what releases the
// operation for deletion and for TTL garbage collection.
func TestHandleTerminal_DelegatedDefersTermination(t *testing.T) {
	for name, tc := range terminalHandlers {
		t.Run(name, func(t *testing.T) {
			op := newOp()
			op.Labels = map[string]string{tc.hook + "test": "delegate-a"}

			beacons := &fakeBeaconClient{}
			adapter := &stubAdapter{}
			h := &handler{beacons: beacons, dynamic: &fakeDynamic{}}
			s := newScope(op, newBeacon(ops.BeaconOwnerKey(OperationKind, op), true), adapter)

			// The outcome the phase handler recorded before delegating. It is the only record of
			// why the operation ended, so delegating must not overwrite it — the delegate is
			// reported on Finalized by updateStatus instead.
			status := opv1alpha1.EncryptionKeyRotationStatus{}
			tc.cond.True(&status)
			tc.cond.Reason(&status, opv1alpha1.PlanFailedReason)
			tc.cond.Message(&status, "the operative detail")

			got, err := tc.handle(h, s, status)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.cond.GetReason(&got) != opv1alpha1.PlanFailedReason {
				t.Fatalf("the outcome reason must survive the delegation, got %q", tc.cond.GetReason(&got))
			}
			if tc.cond.GetMessage(&got) != "the operative detail" {
				t.Fatalf("the outcome message must survive the delegation, got %q", tc.cond.GetMessage(&got))
			}
			if !got.TerminatedAt.IsZero() {
				t.Fatal("terminal handling is still delegated, so it must not be recorded as complete")
			}
			if len(adapter.pauseCalls) != 0 {
				t.Fatalf("the cluster must stay paused while the delegate works, got %v", adapter.pauseCalls)
			}
		})
	}
}

// --- deletion --------------------------------------------------------------------------------

func TestOnChange_DeletingWithoutOurFinalizerIsSkipped(t *testing.T) {
	op := newDeletingOp()
	op.Finalizers = nil

	h, controller, beacons, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(controller.updates) != 0 {
		t.Fatal("an operation we never finalized has no teardown left to run")
	}
	if len(beacons.statusUpdates) != 0 {
		t.Fatal("the beacon must not be touched")
	}
	if controller.enqueueCalls != 0 {
		t.Fatal("an operation deleting under someone else's finalizer must not be polled")
	}
}

// TestOnChange_DeletionCancelsInFlightOperation covers the core of the deletion contract: an
// operation deleted while it is still running is canceled, the cluster it paused is unpaused, its
// beacon is released, and only then is the finalizer retired — one reconcile later, so the canceled
// status is persisted for observers before the object is allowed to disappear.
func TestOnChange_DeletionCancelsInFlightOperation(t *testing.T) {
	op := newDeletingOp()
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, controller, beacons, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("expected phase Canceled, got %q", status.Phase)
	}
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.OperationDeletedReason {
		t.Fatalf("expected reason %q, got %q", opv1alpha1.OperationDeletedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("handleCanceled ran to completion, so it must be recorded")
	}
	if len(beacons.statusUpdates) == 0 || beacons.statusUpdates[len(beacons.statusUpdates)-1].Status.Owner != "" {
		t.Fatal("the beacon must be released on the way out")
	}
	if len(controller.updates) != 0 {
		t.Fatal("the finalizer must outlive the status write that records the cancellation")
	}

	// Second pass, with the status the first pass returned now persisted: nothing moves, so the
	// finalizer can go and the API server can finish the deletion.
	op.Status = status

	status, err = h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("the recorded outcome must not change, got %q", status.Phase)
	}
	if len(controller.updates) != 1 {
		t.Fatalf("expected the finalizer to be removed once, got %d updates", len(controller.updates))
	}
	if slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("expected the finalizer to be removed")
	}
	if controller.deleteCalls != 0 {
		t.Fatal("the object is already deleting; TTL cleanup must not fire")
	}
}

// TestOnChange_DeletionWaitsForTerminalHook is the case the finalizer exists for: the operation is
// deleted while a canceled phase hook delegate holds the beacon on its behalf. The operation must
// be kept alive — the delegate's ownership is anchored to it — until the delegate is done.
func TestOnChange_DeletionWaitsForTerminalHook(t *testing.T) {
	op := newDeletingOp()
	op.Labels = map[string]string{opv1alpha1.CanceledPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, controller, _, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("expected phase Canceled, got %q", status.Phase)
	}
	// The cancellation reason is the operation's outcome and must not be displaced by the delegate;
	// the delegate is reported on Finalized, which is the thing still outstanding.
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.OperationDeletedReason {
		t.Fatalf("expected the cancellation reason to survive, got %q", opv1alpha1.CanceledCondition.GetReason(&status))
	}
	if opv1alpha1.FinalizedCondition.GetReason(&status) != opv1alpha1.WaitingForDelegateReason {
		t.Fatalf("expected Finalized to report the delegate, got %q", opv1alpha1.FinalizedCondition.GetReason(&status))
	}
	if !strings.Contains(opv1alpha1.FinalizedCondition.GetMessage(&status), "delegate-a") {
		t.Fatalf("expected the delegate in the Finalized message, got %q", opv1alpha1.FinalizedCondition.GetMessage(&status))
	}
	if !status.TerminatedAt.IsZero() {
		t.Fatal("the delegate still holds the beacon, so termination must not be recorded")
	}
	if len(controller.updates) != 0 {
		t.Fatal("the finalizer must be held while the delegate works")
	}

	// Stable pass while the delegate works: the operation is polled, not released.
	op.Status = status

	status, err = h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error on stable pass: %v", err)
	}
	if !status.TerminatedAt.IsZero() {
		t.Fatal("termination must still be withheld")
	}
	if len(controller.updates) != 0 {
		t.Fatal("the finalizer must be held until the delegate hands the beacon back")
	}
	if controller.enqueueCalls != 1 {
		t.Fatalf("expected the operation to keep polling, got %d enqueues", controller.enqueueCalls)
	}

	// The delegate finishes and drops its hook label; terminal handling can now complete.
	op.Labels = nil

	status, err = h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error after hook cleared: %v", err)
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("the beacon has been released, so termination must be recorded")
	}
	if string(opv1alpha1.FinalizedCondition.GetStatus(&status)) != "True" {
		t.Fatal("the delegate is gone, so the wait must resolve")
	}
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.OperationDeletedReason {
		t.Fatalf("the outcome reason must still be the one recorded at cancellation, got %q", opv1alpha1.CanceledCondition.GetReason(&status))
	}

	op.Status = status

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error on final pass: %v", err)
	}
	if len(controller.updates) != 1 || slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("expected the finalizer to be removed once terminal handling was recorded")
	}
}

// TestOnChange_DeletionPreservesTerminatedOutcome guards the other half of the rule: cancellation is
// only for operations deleted *before* their terminal handling completed. An operation which
// finished its work keeps the phase it finished in.
func TestOnChange_DeletionPreservesTerminatedOutcome(t *testing.T) {
	op := newDeletingOp()
	initial := opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.EncryptionKeyRotationStepRestart,
	}
	initial.SetTerminated()
	op.Status = updateStatus(op, initial)

	h, controller, _, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseSucceeded {
		t.Fatalf("a terminated operation must not be demoted to Canceled, got %q", status.Phase)
	}
	if string(opv1alpha1.SucceededCondition.GetStatus(&status)) != "True" {
		t.Fatal("the outcome it finished with must be asserted")
	}
	if string(opv1alpha1.CanceledCondition.GetStatus(&status)) != "False" {
		t.Fatal("the Canceled condition must be denied, not raised")
	}
	if len(controller.updates) != 1 || slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("terminal handling was already complete, so the finalizer can go immediately")
	}
}

// TestOnChange_DeletionOfPausedOperation covers a paused operation being deleted. Pausing stops the
// controller touching the operation at all, and tearing it down is no exception: its beacon is left
// alone and its finalizer stays, so the deletion waits for the pause to lift. Resuming the
// operation is what lets it finish deleting.
func TestOnChange_DeletionOfPausedOperation(t *testing.T) {
	op := newDeletingOp()
	op.Spec.Paused = true
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, controller, beacons, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseInProgress {
		t.Fatalf("a paused operation is not reconciled, even to cancel it: phase is %q", status.Phase)
	}
	if string(opv1alpha1.PausedCondition.GetStatus(&status)) != "True" {
		t.Fatal("its conditions are still refreshed")
	}
	if len(beacons.statusUpdates) != 0 {
		t.Fatal("the beacon must be left exactly as the pause found it")
	}
	if len(controller.updates) != 0 {
		t.Fatal("the finalizer must stay until the operation is resumed")
	}

	// Resumed, the deletion proceeds as it would have in the first place.
	op.Spec.Paused = false
	op.Status = status

	status, err = h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error once resumed: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("expected phase Canceled once resumed, got %q", status.Phase)
	}
	if len(beacons.statusUpdates) == 0 {
		t.Fatal("the beacon is released once the operation is resumed")
	}

	op.Status = status

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error on the final pass: %v", err)
	}
	if len(controller.updates) != 1 || slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("the finalizer goes once terminal handling completes")
	}
}

// TestOnChange_DeletionWithMissingBeacon covers deleting an operation whose beacon has already been
// collected: there is nothing left to release, so the operation must not sit in Terminating waiting
// for a beacon that will never come back.
func TestOnChange_DeletionWithMissingBeacon(t *testing.T) {
	op := newDeletingOp()
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, controller, _, _ := newOnChangeHandler(nil)

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("expected phase Canceled, got %q", status.Phase)
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("with no beacon there is nothing to release")
	}

	op.Status = status

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}
	if len(controller.updates) != 1 || slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("expected the finalizer to be removed")
	}
}

// --- finalizer -------------------------------------------------------------------------------

func TestOnChange_TakesFinalizer(t *testing.T) {
	// The operation starts out Pending, which keeps the reconcile in handlePending (waiting for
	// system-agents to register) rather than dispatching plans the fake has no secrets for.
	op := newOnChangeOp()

	h, controller, _, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(controller.updates) != 1 {
		t.Fatalf("the finalizer has to be written with an explicit Update, got %d updates", len(controller.updates))
	}
	if !slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("expected the finalizer to be added")
	}
	if !slices.Contains(op.Finalizers, Finalizer) {
		t.Fatal("the object the status handler goes on to update must reflect the write, or its UpdateStatus conflicts")
	}

	// Subsequent reconciles must not keep writing it.
	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}
	if len(controller.updates) != 1 {
		t.Fatalf("taking the finalizer must be idempotent, got %d updates", len(controller.updates))
	}
}

// TestOnChange_PausedOperationDoesNotTakeFinalizer follows from a paused operation not being
// reconciled at all. It matters most for one which was paused before it ever ran: having dispatched
// nothing, it has nothing to tear down, and a finalizer would only stand between the user and
// deleting it.
func TestOnChange_PausedOperationDoesNotTakeFinalizer(t *testing.T) {
	op := newOnChangeOp()
	op.Spec.Paused = true

	h, controller, _, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(controller.updates) != 0 {
		t.Fatal("a paused operation must not take the finalizer")
	}
	if len(op.Finalizers) != 0 {
		t.Fatal("a paused operation must not take the finalizer")
	}
}

// --- conditions ------------------------------------------------------------------------------

// TestUpdateStatusFinalizedOnlyOnceTerminated pins the split between the two questions a waiter
// can ask. The outcome is asserted the moment the operation reaches its terminal phase, so
// `kubectl wait --for=condition=Succeeded` unblocks as soon as the work is done; Finalized is the
// one that waits for the controller to be finished with the operation, so pairing the two means
// "succeeded and fully wrapped up".
func TestUpdateStatusFinalizedOnlyOnceTerminated(t *testing.T) {
	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhasePending,
		opv1alpha1.OperationPhaseInProgress,
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
		opv1alpha1.OperationPhaseCanceled,
	} {
		t.Run(string(phase), func(t *testing.T) {
			// Every phase, with the terminal marker deliberately absent — including the terminal
			// phases, which is the window a deletion would cancel.
			got := updateStatus(newOp(), opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
			})

			if string(opv1alpha1.FinalizedCondition.GetStatus(&got)) == "True" {
				t.Fatal("Finalized must not be asserted before terminal handling completes")
			}

			if !ops.IsTerminal(phase) {
				return
			}

			outcome, _ := opv1alpha1.OutcomeConditionFor(phase)
			if string(outcome.GetStatus(&got)) != "True" {
				t.Fatalf("%s must be asserted as soon as the terminal phase is reached", outcome)
			}
		})
	}
}

// TestUpdateStatusTerminatedOutcome covers the other half of the outcome contract: once terminal
// handling is recorded, the matching outcome condition goes True while keeping the reason the phase
// handler gave it, the competing outcomes go False, and Finalized summarises all three.
func TestUpdateStatusTerminatedOutcome(t *testing.T) {
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
			initial := opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: tc.phase},
			}
			initial.SetTerminated()
			// What the phase handler recorded at decision time, which must survive to the end.
			tc.outcome.Reason(&initial, tc.reason)
			tc.outcome.Message(&initial, "the operative detail")

			got := updateStatus(newOp(), initial)

			if string(tc.outcome.GetStatus(&got)) != "True" {
				t.Fatalf("expected %s=True", tc.outcome)
			}
			if tc.outcome.GetReason(&got) != tc.reason {
				t.Fatalf("the decision-time reason must not be overwritten: got %q", tc.outcome.GetReason(&got))
			}
			if tc.outcome.GetMessage(&got) != "the operative detail" {
				t.Fatalf("the decision-time message must not be overwritten: got %q", tc.outcome.GetMessage(&got))
			}
			for _, other := range tc.others {
				if string(other.GetStatus(&got)) != "False" {
					t.Fatalf("%s must be denied once another outcome is asserted", other)
				}
			}
			if string(opv1alpha1.FinalizedCondition.GetStatus(&got)) != "True" {
				t.Fatal("expected Finalized=True")
			}
		})
	}
}

// --- TTL garbage collection ------------------------------------------------------------------

// TestOnChange_ExpiredTerminalOperationIsCollectedOnceTerminated pins the TTL guard to the same
// marker the deletion flow uses. Collecting an operation whose terminal handling had not completed
// would strand whatever still holds its beacon and leave the cluster paused, and the deletion it
// triggers would then rewrite the phase it finished in to Canceled.
func TestOnChange_ExpiredTerminalOperationIsCollectedOnceTerminated(t *testing.T) {
	op := newOnChangeOp()
	op.Finalizers = []string{Finalizer}
	op.Spec.TTL = 0 // expire as soon as the operation is terminal
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "test": "delegate-a"}
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:       opv1alpha1.OperationPhaseSucceeded,
			LastUpdated: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
		Step: opv1alpha1.EncryptionKeyRotationStepRestart,
	}

	h, controller, _, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	// handleSucceeded is delegated, so termination is not recorded and the expired operation stays.
	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.TerminatedAt.IsZero() {
		t.Fatal("termination must be withheld while the hook is delegated")
	}

	op.Status = status

	status, err = h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error on stable pass: %v", err)
	}
	if controller.deleteCalls != 0 {
		t.Fatal("an operation whose terminal handling is unfinished must not be collected")
	}
	if controller.enqueueCalls != 1 {
		t.Fatalf("expected one poll, got %d", controller.enqueueCalls)
	}

	// The delegate finishes: handleSucceeded unpauses, releases the beacon and records termination.
	op.Labels = nil

	status, err = h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error after hook cleared: %v", err)
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("expected termination to be recorded")
	}
	if controller.deleteCalls != 0 {
		t.Fatal("the status moved this pass, so collection waits for it to settle")
	}

	op.Status = status

	if _, err := h.OnChange(op, op.Status); !errors.Is(err, generic.ErrSkip) {
		t.Fatalf("expected ErrSkip for the collected operation, got %v", err)
	}
	if controller.deleteCalls != 1 {
		t.Fatalf("expected the expired terminated operation to be collected once, got %d", controller.deleteCalls)
	}
}

// --- cancellation ----------------------------------------------------------------------------

// TestOnChange_CancelRequestedCancelsInFlightOperation covers the core of the cancellation
// contract, which mirrors the deletion one: an operation canceled while it is still running stops
// where it is, releases the beacon, and reports why it was canceled. The cluster the rotation
// paused is unpaused on the way out, which TestHandleTerminal_RecordsTerminationAndUnpauses covers
// for every terminal phase including this one.
func TestOnChange_CancelRequestedCancelsInFlightOperation(t *testing.T) {
	op := newOnChangeOp()
	op.Spec.Cancel = true
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, _, beacons, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("expected phase Canceled, got %q", status.Phase)
	}
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.CancelRequestedReason {
		t.Fatalf("expected reason %q, got %q", opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("handleCanceled ran to completion, so it must be recorded")
	}
	if len(beacons.statusUpdates) == 0 || beacons.statusUpdates[len(beacons.statusUpdates)-1].Status.Owner != "" {
		t.Fatalf("the beacon must be released, got %+v", beacons.statusUpdates)
	}
}

// TestOnChange_CancelRequestedInTerminalPhaseIsDeclined covers the edge of the window: the
// operation's work is over and its outcome asserted, so there is nothing for a cancellation to
// stop, even though its terminal phase hook is still holding the beacon. Deleting the operation is
// what breaks that deadlock.
func TestOnChange_CancelRequestedInTerminalPhaseIsDeclined(t *testing.T) {
	op := newOnChangeOp()
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "wedged": "delegate-a"}
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.EncryptionKeyRotationStepRestart,
	}

	beacon := newBeacon(ops.BeaconOwnerKey(OperationKind, op), true)
	beacon.Status.Delegates = []string{"delegate-a"}
	h, _, beacons, _ := newOnChangeHandler(beacon)

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseSucceeded {
		t.Fatalf("the operation had already concluded, so it keeps the phase it ended in: got %q", status.Phase)
	}
	if string(opv1alpha1.SucceededCondition.GetStatus(&status)) != "True" {
		t.Fatal("its outcome must survive the request")
	}
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.CancellationDeclinedReason {
		t.Fatalf("a declined cancellation must be acknowledged, got reason %q", opv1alpha1.CanceledCondition.GetReason(&status))
	}
	if !status.TerminatedAt.IsZero() {
		t.Fatal("the hook is still owed an answer, so handling is not complete")
	}
	for _, update := range beacons.statusUpdates {
		if update.Status.Owner != ops.BeaconOwnerKey(OperationKind, op) {
			t.Fatalf("the beacon must stay with the operation and its delegate, got owner %q", update.Status.Owner)
		}
	}
}

// TestOnChange_CancelRequestedAfterTerminationKeepsOutcome is the same rule for an operation the
// controller has fully finished with: nothing left to call off, and by then nothing left to release
// either.
func TestOnChange_CancelRequestedAfterTerminationKeepsOutcome(t *testing.T) {
	op := newOnChangeOp()
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Status = updateStatus(op, opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{
			Phase:        opv1alpha1.OperationPhaseSucceeded,
			TerminatedAt: metav1.Now(),
		},
		Step: opv1alpha1.EncryptionKeyRotationStepRestart,
	})

	h, _, beacons, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseSucceeded {
		t.Fatalf("a terminated operation must not be demoted to Canceled, got %q", status.Phase)
	}
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.CancellationDeclinedReason {
		t.Fatalf("expected the request to be acknowledged, got reason %q", opv1alpha1.CanceledCondition.GetReason(&status))
	}
	if len(beacons.statusUpdates) != 0 {
		t.Fatal("there is nothing left to release")
	}
}

// TestOnChange_PausedBlocksCancel pins the documented precedence: Paused halts reconciliation
// outright, so a cancellation requested on a paused operation is not acted on until it is resumed.
func TestOnChange_PausedBlocksCancel(t *testing.T) {
	op := newOnChangeOp()
	op.Spec.Paused = true
	op.Spec.Cancel = true
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, controller, beacons, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseInProgress {
		t.Fatalf("a paused operation must not transition on cancel, got %q", status.Phase)
	}
	if opv1alpha1.CanceledCondition.GetStatus(&status) != "" {
		t.Fatal("the Canceled condition must not be reported while paused")
	}
	if string(opv1alpha1.PausedCondition.GetStatus(&status)) != "True" {
		t.Fatal("the operation must report that it is paused")
	}
	if len(beacons.statusUpdates) != 0 {
		t.Fatal("the beacon must be left as the pause found it")
	}
	if len(controller.updates) != 0 {
		t.Fatal("a paused operation is not finalized, so it takes no finalizer")
	}
}

// TestUpdateStatusReportsDeclinedCancellation covers the acknowledgement on its own, across every
// outcome an operation can end in.
func TestUpdateStatusReportsDeclinedCancellation(t *testing.T) {
	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
	} {
		t.Run(string(phase), func(t *testing.T) {
			initial := opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
			}

			op := newOp()
			got := updateStatus(op, initial)
			if opv1alpha1.CanceledCondition.GetReason(&got) != opv1alpha1.NotCanceledReason {
				t.Fatalf("with no request to report the denial stays generic, got %q", opv1alpha1.CanceledCondition.GetReason(&got))
			}

			op.Spec.Cancel = true
			got = updateStatus(op, initial)
			if string(opv1alpha1.CanceledCondition.GetStatus(&got)) != "False" {
				t.Fatal("the operation was not canceled, so the condition stays denied")
			}
			if opv1alpha1.CanceledCondition.GetReason(&got) != opv1alpha1.CancellationDeclinedReason {
				t.Fatalf("expected reason %q, got %q", opv1alpha1.CancellationDeclinedReason, opv1alpha1.CanceledCondition.GetReason(&got))
			}
			if !strings.Contains(opv1alpha1.CanceledCondition.GetMessage(&got), string(phase)) {
				t.Fatalf("the message should say which phase was already reached, got %q", opv1alpha1.CanceledCondition.GetMessage(&got))
			}

			outcome, _ := opv1alpha1.OutcomeConditionFor(phase)
			if string(outcome.GetStatus(&got)) != "True" {
				t.Fatal("the request must not disturb the outcome")
			}
		})
	}

	// An operation which really was canceled keeps the reason it was canceled for: the
	// acknowledgement is for requests that were *not* acted on.
	t.Run("canceled keeps its own reason", func(t *testing.T) {
		op := newOp()
		op.Spec.Cancel = true

		status := opv1alpha1.EncryptionKeyRotationStatus{
			OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseCanceled},
		}
		status.MarkCanceled(opv1alpha1.CancelRequestedReason, "cancellation requested")

		got := updateStatus(op, status)
		if string(opv1alpha1.CanceledCondition.GetStatus(&got)) != "True" {
			t.Fatal("the operation was canceled, so the condition must be raised")
		}
		if opv1alpha1.CanceledCondition.GetReason(&got) != opv1alpha1.CancelRequestedReason {
			t.Fatalf("expected the cancellation reason to survive, got %q", opv1alpha1.CanceledCondition.GetReason(&got))
		}
	})
}

// TestFinishRotation covers the one place a rotation unpauses the cluster. reconcileRestart reaches
// it only after every server node has restarted and converged, and the unpause and the success
// marker are done together, so a rotation cannot report success while leaving the cluster frozen —
// nor unpause without having succeeded. Every terminal handler is covered for the opposite by
// TestHandleTerminal_RecordsTermination.
func TestFinishRotation(t *testing.T) {
	adapter := &stubAdapter{}
	h := &handler{}
	s := newScope(newOp(), newBeacon(ops.BeaconOwnerKey(OperationKind, newOp()), true), adapter)

	status, err := h.finishRotation(s, opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRestart,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(adapter.pauseCalls) != 1 || adapter.pauseCalls[0] {
		t.Fatalf("expected exactly one PauseCluster(false), got %v", adapter.pauseCalls)
	}
	if status.Phase != opv1alpha1.OperationPhaseSucceeded {
		t.Fatalf("expected phase Succeeded, got %q", status.Phase)
	}
	if string(opv1alpha1.SucceededCondition.GetStatus(&status)) != "True" {
		t.Fatal("the outcome must be asserted")
	}
}

// A cluster that cannot be unpaused leaves the rotation in progress to be retried, rather than
// reported successful with the cluster still frozen.
func TestFinishRotation_UnpauseFailureDoesNotSucceed(t *testing.T) {
	adapter := &stubAdapter{pauseErr: errors.New("boom")}
	h := &handler{}
	s := newScope(newOp(), newBeacon(ops.BeaconOwnerKey(OperationKind, newOp()), true), adapter)

	status, err := h.finishRotation(s, opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRestart,
	})
	if err == nil {
		t.Fatal("expected the unpause failure to be returned")
	}
	if status.Phase == opv1alpha1.OperationPhaseSucceeded {
		t.Fatal("the rotation must not be reported successful while the cluster is still paused")
	}
}

// TestHandlePending_ReclaimsSupersededClaim covers the wiring of the no-lookup reclaim. A beacon
// still carrying a claim from an earlier object of this operation's name would otherwise leave the
// operation waiting on a holder that no longer exists, and the delegate that claim left behind —
// pushed to gate a hook that died with the object, so nothing will ever pop it — would be handed to
// the operation that acquires next.
func TestHandlePending_ReclaimsSupersededClaim(t *testing.T) {
	op := newOp()
	superseded := ops.BeaconOwnerKey(OperationKind, &opv1alpha1.EncryptionKeyRotation{
		ObjectMeta: metav1.ObjectMeta{Namespace: op.Namespace, Name: op.Name, UID: "dead-uid"},
	})

	beacon := newBeacon(superseded, true)
	beacon.Status.Delegates = []string{"dead-delegate"}

	beacons := &fakeBeaconClient{beacon: beacon}
	h := &handler{
		beacons: beacons,
		// The claim carries this operation's own name under a dead uid, which is provable without
		// asking the API server anything: acquiring a beacon reads no operations.
		encryptionkeyrotations: &fakeEncryptionKeyRotationController{
			getFn: func(namespace, name string, _ metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
				t.Fatalf("unexpected lookup of %s/%s", namespace, name)
				return nil, nil
			},
		},
	}
	s := newScope(op, beacon, &stubAdapter{})

	if _, err := h.handlePending(s, opv1alpha1.EncryptionKeyRotationStatus{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.beacon.Status.Owner != ops.BeaconOwnerKey(OperationKind, op) {
		t.Fatalf("the operation must end up holding the beacon, got %q", s.beacon.Status.Owner)
	}
	if len(s.beacon.Status.Delegates) != 0 {
		t.Fatalf("the dead claim's delegate must not be inherited, got %v", s.beacon.Status.Delegates)
	}
}

// missingClusterOp points an operation at a cluster the dynamic resolver does not serve, which is
// what the reconcile sees once a cluster has been deleted underneath an operation.
func missingClusterOp(op *opv1alpha1.EncryptionKeyRotation) *opv1alpha1.EncryptionKeyRotation {
	op.Spec.ClusterRef.Name = "gone"
	return op
}

// --- a missing cluster or beacon for an operation which has already concluded ------------------

// TestOnChange_CancelWithMissingClusterKeepsOutcome covers the sequence that motivated this rule:
// cancellation is applied before the scope is resolved, so a canceled operation whose cluster is
// gone reaches the missing-cluster branch already terminal. Failing it there would report the
// user's cancellation as a failure.
func TestOnChange_CancelWithMissingClusterKeepsOutcome(t *testing.T) {
	op := missingClusterOp(newOnChangeOp())
	op.Spec.Cancel = true
	op.Spec.TTL = -1
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
		Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
	}

	h, _, _, _ := newOnChangeHandler(newBeacon(ops.BeaconOwnerKey(OperationKind, op), true))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseCanceled {
		t.Fatalf("the cancellation must not be reported as a failure, got phase %q", status.Phase)
	}
	if opv1alpha1.CanceledCondition.GetReason(&status) != opv1alpha1.CancelRequestedReason {
		t.Fatalf("expected reason %q, got %q", opv1alpha1.CancelRequestedReason, opv1alpha1.CanceledCondition.GetReason(&status))
	}
	if opv1alpha1.FailedCondition.GetReason(&status) == opv1alpha1.ClusterNotFoundReason {
		t.Fatal("the missing cluster must not be recorded as the outcome")
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("with no cluster there is nothing to release")
	}
	if string(opv1alpha1.FinalizedCondition.GetStatus(&status)) != "True" {
		t.Fatal("nothing is owed, so the operation is finalized")
	}
}

// The same rule protects an operation which concluded on an earlier reconcile: deleting the cluster
// inside the operation's TTL must not rewrite what it ended with.
func TestOnChange_MissingClusterKeepsConcludedOutcome(t *testing.T) {
	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhaseSucceeded,
		opv1alpha1.OperationPhaseFailed,
		opv1alpha1.OperationPhaseAborted,
		opv1alpha1.OperationPhaseCanceled,
	} {
		t.Run(string(phase), func(t *testing.T) {
			op := missingClusterOp(newOnChangeOp())
			op.Spec.TTL = -1

			initial := opv1alpha1.EncryptionKeyRotationStatus{Step: opv1alpha1.EncryptionKeyRotationStepRestart}
			initial.SetPhase(phase)
			op.Status = initial

			h, _, _, _ := newOnChangeHandler(newBeacon("", false))

			status, err := h.OnChange(op, op.Status)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.Phase != phase {
				t.Fatalf("the phase the operation ended in must stand, got %q", status.Phase)
			}
			outcome, _ := opv1alpha1.OutcomeConditionFor(phase)
			if string(outcome.GetStatus(&status)) != "True" {
				t.Fatalf("expected %s to be asserted", outcome)
			}
			if status.TerminatedAt.IsZero() {
				t.Fatal("with no cluster there is nothing to release")
			}
		})
	}
}

// TestOnChange_MissingClusterWithOwedHookDefersTermination is the other half of the rule. The
// operation cannot be handed a beacon that is not there, but its terminal phase hook is still
// labelled: the delegate has not had its turn, so recording termination would claim it had.
func TestOnChange_MissingClusterWithOwedHookDefersTermination(t *testing.T) {
	op := missingClusterOp(newOnChangeOp())
	op.Spec.TTL = -1
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}

	initial := opv1alpha1.EncryptionKeyRotationStatus{Step: opv1alpha1.EncryptionKeyRotationStepRestart}
	initial.MarkSucceeded()
	op.Status = initial

	h, _, _, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhaseSucceeded {
		t.Fatalf("expected the outcome to stand, got %q", status.Phase)
	}
	if !status.TerminatedAt.IsZero() {
		t.Fatal("the delegate has not had its turn, so nothing may be recorded")
	}
	if opv1alpha1.FinalizedCondition.GetReason(&status) != opv1alpha1.WaitingForDelegateReason {
		t.Fatalf("the wait must be reported against the delegate holding it up, got %q",
			opv1alpha1.FinalizedCondition.GetReason(&status))
	}
}

// A deleting operation is the deliberate exception: it is being discarded and its hooks go with it,
// so it terminates at once rather than holding its finalizer open for a delegate that will never be
// handed anything.
func TestOnChange_DeletionWithMissingClusterAbandonsOwedHook(t *testing.T) {
	op := missingClusterOp(newDeletingOp())
	op.Labels = map[string]string{opv1alpha1.SucceededPhaseHookLabelPrefix + "verify": "delegate-a"}
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
		Step:            opv1alpha1.EncryptionKeyRotationStepRestart,
	}

	h, controller, _, _ := newOnChangeHandler(newBeacon("", false))

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.TerminatedAt.IsZero() {
		t.Fatal("a deletion abandons the hook rather than waiting on it")
	}

	op.Status = status

	if _, err := h.OnChange(op, op.Status); err != nil {
		t.Fatalf("unexpected error on the second pass: %v", err)
	}
	if len(controller.updates) != 1 || slices.Contains(controller.updates[0].Finalizers, Finalizer) {
		t.Fatal("the finalizer must not be held for an abandoned hook")
	}
}

// TestOnChange_MissingBeaconDisposition covers what an operation does when the beacon it needs is
// not there. What it should do depends entirely on what it still owes: an outcome which dispatched
// work owes a release and complains that it cannot make one, while an outcome which dispatched
// nothing owes nothing and finishes.
func TestOnChange_MissingBeaconDisposition(t *testing.T) {
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
			op := newOnChangeOp()
			op.Spec.TTL = -1

			initial := opv1alpha1.EncryptionKeyRotationStatus{Step: opv1alpha1.EncryptionKeyRotationStepRestart}
			initial.SetPhase(tc.phase)
			if tc.terminated {
				initial.SetTerminated()
			}
			op.Status = initial

			h, _, _, _ := newOnChangeHandler(nil)

			status, err := h.OnChange(op, op.Status)
			if tc.wantErr && err == nil {
				t.Fatal("the operation must complain about a beacon it cannot release")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// A handler which returns an error has its status reverted by the generated status
			// handler, so only the phase it was already in is observable on that path.
			if status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q", status.Phase, tc.wantPhase)
			}
			if tc.wantReason != "" {
				outcome, _ := opv1alpha1.OutcomeConditionFor(status.Phase)
				if outcome.GetReason(&status) != tc.wantReason {
					t.Fatalf("reason = %q, want %q", outcome.GetReason(&status), tc.wantReason)
				}
			}
			if terminated := !status.TerminatedAt.IsZero(); terminated != tc.wantTerminated {
				t.Fatalf("terminated = %v, want %v: termination is recorded only when nothing is owed",
					terminated, tc.wantTerminated)
			}
		})
	}
}

// A Pending operation has not acquired the beacon yet, so its absence is "not created" rather than
// "lost" — the system-agent controller creates one once the cluster can take operations — and the
// operation waits rather than failing.
func TestOnChange_MissingBeaconWhilePendingWaits(t *testing.T) {
	op := newOnChangeOp()
	op.Spec.TTL = -1
	op.Status = opv1alpha1.EncryptionKeyRotationStatus{
		OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhasePending},
	}

	h, _, _, _ := newOnChangeHandler(nil)

	status, err := h.OnChange(op, op.Status)
	if err != nil {
		t.Fatalf("a beacon yet to be created is not a failure: %v", err)
	}
	if status.Phase != opv1alpha1.OperationPhasePending {
		t.Fatalf("phase = %q, want Pending", status.Phase)
	}
	if opv1alpha1.PendingCondition.GetReason(&status) != opv1alpha1.WaitingForBeaconReason {
		t.Fatalf("reason = %q, want %q", opv1alpha1.PendingCondition.GetReason(&status), opv1alpha1.WaitingForBeaconReason)
	}
	if !status.TerminatedAt.IsZero() {
		t.Fatal("nothing has been handled yet, so nothing may be recorded")
	}
}

// TestOnChange_MissingClusterStillFailsRunningOperation guards the rule from over-reaching: for an
// operation which still has work to dispatch, a missing cluster is exactly the failure it always
// was.
func TestOnChange_MissingClusterStillFailsRunningOperation(t *testing.T) {
	for _, phase := range []opv1alpha1.OperationPhase{
		opv1alpha1.OperationPhasePending,
		opv1alpha1.OperationPhaseInProgress,
	} {
		t.Run(string(phase), func(t *testing.T) {
			op := missingClusterOp(newOnChangeOp())
			op.Spec.TTL = -1
			op.Status = opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: phase},
				Step:            opv1alpha1.EncryptionKeyRotationStepRotate,
			}

			h, _, _, _ := newOnChangeHandler(newBeacon("", false))

			status, err := h.OnChange(op, op.Status)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.Phase != opv1alpha1.OperationPhaseFailed {
				t.Fatalf("expected phase Failed, got %q", status.Phase)
			}
			if opv1alpha1.FailedCondition.GetReason(&status) != opv1alpha1.ClusterNotFoundReason {
				t.Fatalf("expected reason %q, got %q", opv1alpha1.ClusterNotFoundReason, opv1alpha1.FailedCondition.GetReason(&status))
			}
			if status.TerminatedAt.IsZero() {
				t.Fatal("there is no beacon to release, so the failure must not be left uncollectable")
			}
		})
	}
}
