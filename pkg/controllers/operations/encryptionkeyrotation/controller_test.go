package encryptionkeyrotation

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"testing"

	opv1alpha1 "github.com/rancher/rancher/pkg/apis/operation.cattle.io/v1alpha1"
	operationcontrollers "github.com/rancher/rancher/pkg/generated/controllers/operation.cattle.io/v1alpha1"
	planapi "github.com/rancher/rancher/pkg/plan"
	planv1alpha1 "github.com/rancher/rancher/pkg/plan/api/plan.cattle.io/v1alpha1"
	plancontrollers "github.com/rancher/rancher/pkg/plan/generated/controllers/plan.cattle.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

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
						planv1alpha1.OwnerLabel: beaconOwnerKey(currentOp),
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
						planv1alpha1.OwnerLabel: "etcd-snapshot-save",
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
						planv1alpha1.OwnerLabel: "encryption-key-rotation-old-owner",
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
						planv1alpha1.OwnerLabel: "encryption-key-rotation-old-owner",
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
						planv1alpha1.OwnerLabel: "encryption-key-rotation-old-owner",
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
						planv1alpha1.OwnerLabel: "encryption-key-rotation-old-owner",
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
						planv1alpha1.OwnerLabel: "encryption-key-rotation-old-owner",
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

			owner := s.beacon.Labels[planv1alpha1.OwnerLabel]
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
	updateCalls int
}

func (f *fakeBeaconClient) Update(beacon *planv1alpha1.Beacon) (*planv1alpha1.Beacon, error) {
	f.updateCalls++
	return beacon, nil
}

func newPeriodicStatusSecret(secretName, stdout string) *corev1.Secret {
	periodicOutput := map[string]planapi.PeriodicInstructionOutput{
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
