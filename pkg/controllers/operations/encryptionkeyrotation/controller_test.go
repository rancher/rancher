package encryptionkeyrotation

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	rkeplan "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1/plan"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	ops "github.com/rancher/rancher/pkg/operations"
	"github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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
}

func (a *stubAdapter) WaitForRegister() (bool, error) {
	return a.waitForRegisterOK, a.waitForRegisterErr
}

func (a *stubAdapter) RuntimeCommand() string {
	return "rke2"
}

func (a *stubAdapter) DistroDataDirectory(_ *corev1.Secret) string {
	return "/var/lib/rancher/rke2"
}

func (a *stubAdapter) ProvisioningDataDirectory(_ *corev1.Secret) string {
	return "/var/lib/rancher/capr"
}
func (a *stubAdapter) ServerUnit() string {
	return "rke2-server"
}

func (a *stubAdapter) RenderProbes(_ *corev1.Secret, _ bool) (map[string]rkeplan.Probe, error) {
	return map[string]rkeplan.Probe{}, nil
}

func (a *stubAdapter) KubectlPath(_ *corev1.Secret) string {
	return "/var/lib/rancher/rke2/bin/kubectl"
}

func (a *stubAdapter) KubeconfigPath(_ *corev1.Secret) string {
	return "/etc/rancher/rke2/rke2.yaml"
}

func (a *stubAdapter) FindOrElectLeader(_ string, _ ops.Filter) (*corev1.Secret, error) {
	return nil, nil
}

func (a *stubAdapter) PauseCluster(paused bool) error {
	a.pauseCalls = append(a.pauseCalls, paused)
	return nil
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

func (d *fakeDynamic) Get(_ schema.GroupVersionKind, _, _ string) (runtime.Object, error) {
	if d.getErr != nil {
		return nil, d.getErr
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
	labels := map[string]string{}
	if owner != "" {
		labels[planv1alpha1.BeaconOwnerLabel] = owner
	}
	return &planv1alpha1.Beacon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fleet-default",
			Namespace: "fleet-default",
			Labels:    labels,
		},
		Status: planv1alpha1.BeaconStatus{Active: active},
	}
}

func newScope(op *opv1alpha1.EncryptionKeyRotation, beacon *planv1alpha1.Beacon, adapter ops.Adapter) *scope {
	cluster := &unstructured.Unstructured{}
	cluster.SetAPIVersion("provisioning.cattle.io/v1")
	cluster.SetKind("Cluster")
	cluster.SetNamespace("fleet-default")
	cluster.SetName("test")
	return &scope{
		op:         op,
		beacon:     beacon,
		namespace:  "fleet-default",
		clusterObj: cluster,
		adapter:    adapter,
	}
}

type fakeEncryptionKeyRotationController struct {
	operationcontrollers.EncryptionKeyRotationController
	getFn func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error)
}

func (f *fakeEncryptionKeyRotationController) Get(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
	if f.getFn == nil {
		return nil, nil
	}
	return f.getFn(namespace, name, opts)
}

type fakeBeaconClient struct {
	plancontrollers.BeaconClient
	updateCalls     int
	updates         []*planv1alpha1.Beacon
	statusUpdates   []*planv1alpha1.Beacon
	updateErr       error
	updateStatusErr error
}

func (f *fakeBeaconClient) Update(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.updateCalls++
	f.updates = append(f.updates, beacon.DeepCopy())
	return beacon, f.updateErr
}

func (f *fakeBeaconClient) UpdateStatus(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.statusUpdates = append(f.statusUpdates, beacon.DeepCopy())
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
		name  string
		phase opv1alpha1.OperationPhase
		check func(t *testing.T, status opv1alpha1.EncryptionKeyRotationStatus)
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
			name:  "succeeded clears pending in-progress and failed",
			phase: opv1alpha1.OperationPhaseSucceeded,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.PendingCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected PendingCondition=False")
				}
				if string(opv1alpha1.InProgressCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected InProgressCondition=False")
				}
				if string(opv1alpha1.FailedCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected FailedCondition=False")
				}
				if opv1alpha1.FailedCondition.GetReason(&s) != opv1alpha1.NotFailedReason {
					t.Fatalf("expected FailedCondition reason %q, got %q", opv1alpha1.NotFailedReason, opv1alpha1.FailedCondition.GetReason(&s))
				}
			},
		},
		{
			name:  "failed clears pending in-progress and succeeded",
			phase: opv1alpha1.OperationPhaseFailed,
			check: func(t *testing.T, s opv1alpha1.EncryptionKeyRotationStatus) {
				if string(opv1alpha1.PendingCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected PendingCondition=False")
				}
				if string(opv1alpha1.InProgressCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected InProgressCondition=False")
				}
				if string(opv1alpha1.SucceededCondition.GetStatus(&s)) != "False" {
					t.Fatalf("expected SucceededCondition=False")
				}
				if opv1alpha1.SucceededCondition.GetReason(&s) != opv1alpha1.NotSuccessfulReason {
					t.Fatalf("expected SucceededCondition reason %q, got %q", opv1alpha1.NotSuccessfulReason, opv1alpha1.SucceededCondition.GetReason(&s))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := newOp()
			op.Generation = 42

			status := updateStatus(op, opv1alpha1.EncryptionKeyRotationStatus{
				OperationStatus: opv1alpha1.OperationStatus{Phase: tt.phase},
			})
			if status.ObservedGeneration != 42 {
				t.Fatalf("expected ObservedGeneration=42, got %d", status.ObservedGeneration)
			}
			tt.check(t, status)
		})
	}
}

func TestHandleFailed_HoldingBeaconReleasesAndUnpauses(t *testing.T) {
	op := newOp()
	adapter := &stubAdapter{waitForRegisterOK: true}
	beacons := &fakeBeaconClient{}

	h := &handler{beacons: beacons}
	s := newScope(op, newBeacon(beaconOwnerKey(op), true), adapter)

	_, err := h.handleFailed(s, opv1alpha1.EncryptionKeyRotationStatus{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(adapter.pauseCalls) != 1 || adapter.pauseCalls[0] {
		t.Fatalf("expected PauseCluster(false), got %+v", adapter.pauseCalls)
	}
	if len(beacons.updates) != 1 {
		t.Fatalf("expected one beacon release update, got %d", len(beacons.updates))
	}
	if _, ok := beacons.updates[0].Labels[planv1alpha1.BeaconOwnerLabel]; ok {
		t.Fatalf("expected owner label to be cleared on release")
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
	s := newScope(op, newBeacon(beaconOwnerKey(op), true), adapter)

	_, err := h.handleSucceeded(s, opv1alpha1.EncryptionKeyRotationStatus{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(adapter.pauseCalls) != 1 || adapter.pauseCalls[0] {
		t.Fatalf("expected PauseCluster(false), got %+v", adapter.pauseCalls)
	}
	if len(beacons.statusUpdates) != 1 {
		t.Fatalf("expected one beacon status update, got %d", len(beacons.statusUpdates))
	}
	if beacons.statusUpdates[0].Status.Active {
		t.Fatalf("expected beacon to be toggled inactive")
	}
	if len(beacons.updates) != 1 {
		t.Fatalf("expected one beacon release update, got %d", len(beacons.updates))
	}
	if _, ok := beacons.updates[0].Labels[planv1alpha1.BeaconOwnerLabel]; ok {
		t.Fatalf("expected owner label to be cleared on release")
	}

	if len(dynamic.enqueueCalls) != 1 {
		t.Fatalf("expected one cluster enqueue, got %d", len(dynamic.enqueueCalls))
	}
	expectedGVK := schema.FromAPIVersionAndKind("provisioning.cattle.io/v1", "Cluster")
	if dynamic.enqueueCalls[0].gvk != expectedGVK || dynamic.enqueueCalls[0].namespace != "fleet-default" || dynamic.enqueueCalls[0].name != "test" {
		t.Fatalf("unexpected enqueue call: %#v", dynamic.enqueueCalls[0])
	}
}

func TestHandleSucceeded_NotHoldingOnlyUnpauses(t *testing.T) {
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

	if len(adapter.pauseCalls) != 1 || adapter.pauseCalls[0] {
		t.Fatalf("expected PauseCluster(false), got %+v", adapter.pauseCalls)
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

	s := newScope(op, newBeacon(beaconOwnerKey(op), false), nil)
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

func TestReclaimStaleBeaconOwnerIfNeeded(t *testing.T) {
	currentOp := &opv1alpha1.EncryptionKeyRotation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      "ekr-current",
			UID:       types.UID("current-uid"),
		},
	}

	tests := []struct {
		name             string
		beacon           *planv1alpha1.Beacon
		getFn            func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error)
		wantUpdate       bool
		wantErr          bool
		wantOwnerCleared bool
		wantRefCleared   bool
	}{
		{
			name: "no owner label does not reclaim",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels:    map[string]string{},
				},
			},
		},
		{
			name: "current op owner key does not reclaim",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: beaconOwnerKey(currentOp),
					},
				},
			},
		},
		{
			name: "non matching owner does not reclaim",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: "etcd-snapshot-save",
					},
				},
			},
		},
		{
			name: "malformed owner ref reclaims beacon",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: "encryption-key-rotation-old-owner",
					},
					Annotations: map[string]string{
						beaconOwnerRefAnnotation: "bad-owner-ref",
					},
				},
			},
			wantUpdate:       true,
			wantOwnerCleared: true,
			wantRefCleared:   true,
		},
		{
			name: "missing owner object reclaims beacon",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: "encryption-key-rotation-old-owner",
					},
					Annotations: map[string]string{
						beaconOwnerRefAnnotation: "fleet-default/ekr-old/old-uid",
					},
				},
			},
			getFn: func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
				return nil, apierrors.NewNotFound(schema.GroupResource{Group: "operation.cattle.io", Resource: "encryptionkeyrotations"}, name)
			},
			wantUpdate:       true,
			wantOwnerCleared: true,
			wantRefCleared:   true,
		},
		{
			name: "uid mismatch reclaims beacon",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: "encryption-key-rotation-old-owner",
					},
					Annotations: map[string]string{
						beaconOwnerRefAnnotation: "fleet-default/ekr-old/old-uid",
					},
				},
			},
			getFn: func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
				return &opv1alpha1.EncryptionKeyRotation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       types.UID("different-uid"),
					},
					Status: opv1alpha1.EncryptionKeyRotationStatus{
						OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
					},
				}, nil
			},
			wantUpdate:       true,
			wantOwnerCleared: true,
			wantRefCleared:   true,
		},
		{
			name: "terminal owner reclaims beacon",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: "encryption-key-rotation-old-owner",
					},
					Annotations: map[string]string{
						beaconOwnerRefAnnotation: "fleet-default/ekr-old/old-uid",
					},
				},
			},
			getFn: func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
				return &opv1alpha1.EncryptionKeyRotation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       types.UID("old-uid"),
					},
					Status: opv1alpha1.EncryptionKeyRotationStatus{
						OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseSucceeded},
					},
				}, nil
			},
			wantUpdate:       true,
			wantOwnerCleared: true,
			wantRefCleared:   true,
		},
		{
			name: "active matching owner does not reclaim",
			beacon: &planv1alpha1.Beacon{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-default",
					Namespace: "fleet-default",
					Labels: map[string]string{
						planv1alpha1.BeaconOwnerLabel: "encryption-key-rotation-old-owner",
					},
					Annotations: map[string]string{
						beaconOwnerRefAnnotation: "fleet-default/ekr-old/old-uid",
					},
				},
			},
			getFn: func(namespace, name string, opts metav1.GetOptions) (*opv1alpha1.EncryptionKeyRotation, error) {
				return &opv1alpha1.EncryptionKeyRotation{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      name,
						UID:       types.UID("old-uid"),
					},
					Status: opv1alpha1.EncryptionKeyRotationStatus{
						OperationStatus: opv1alpha1.OperationStatus{Phase: opv1alpha1.OperationPhaseInProgress},
					},
				}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			beaconClient := &fakeBeaconClient{}
			controller := &fakeEncryptionKeyRotationController{getFn: tt.getFn}
			h := &handler{
				beacons:                beaconClient,
				encryptionkeyrotations: controller,
			}
			s := &scope{
				op:     currentOp,
				beacon: tt.beacon.DeepCopy(),
			}

			err := h.reclaimStaleBeaconOwnerIfNeeded(s)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantUpdate && beaconClient.updateCalls == 0 {
				t.Fatalf("expected beacon update")
			}
			if !tt.wantUpdate && beaconClient.updateCalls > 0 {
				t.Fatalf("did not expect beacon update")
			}
			if !tt.wantUpdate {
				return
			}

			owner := s.beacon.Labels[planv1alpha1.BeaconOwnerLabel]
			if tt.wantOwnerCleared && owner != "" {
				t.Fatalf("expected owner label cleared, got %q", owner)
			}
			if s.beacon.Annotations == nil {
				if tt.wantRefCleared {
					return
				}
				t.Fatalf("expected annotations map present")
			}
			if tt.wantRefCleared && s.beacon.Annotations[beaconOwnerRefAnnotation] != "" {
				t.Fatalf("expected owner ref annotation cleared")
			}
		})
	}
}
