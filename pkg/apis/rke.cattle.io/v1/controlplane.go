package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RKEControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              RKEControlPlaneSpec   `json:"spec"`
	Status            RKEControlPlaneStatus `json:"status,omitempty"`
}

type EnvVar struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

type RKEControlPlaneSpec struct {
	RKEClusterSpecCommon

	AgentEnvVars             []EnvVar                 `json:"agentEnvVars,omitempty"`
	LocalClusterAuthEndpoint LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint"`
	ETCDSnapshotCreate       *ETCDSnapshotCreate      `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotRestore      *ETCDSnapshotRestore     `json:"etcdSnapshotRestore,omitempty"`
	RotateCertificates       *RotateCertificates      `json:"rotateCertificates,omitempty"`
	RotateEncryptionKeys     *RotateEncryptionKeys    `json:"rotateEncryptionKeys,omitempty"`
	KubernetesVersion        string                   `json:"kubernetesVersion,omitempty"`
	ClusterName              string                   `json:"clusterName,omitempty" wrangler:"required"`
	ManagementClusterName    string                   `json:"managementClusterName,omitempty" wrangler:"required"`
	UnmanagedConfig          bool                     `json:"unmanagedConfig,omitempty"`
}

type ETCDSnapshotPhase string

const (
	ETCDSnapshotPhaseStarted  ETCDSnapshotPhase = "Started"
	ETCDSnapshotPhaseShutdown ETCDSnapshotPhase = "Shutdown"
	ETCDSnapshotPhaseRestore  ETCDSnapshotPhase = "Restore"
	ETCDSnapshotPhaseFinished ETCDSnapshotPhase = "Finished"
	ETCDSnapshotPhaseFailed   ETCDSnapshotPhase = "Failed"
)

type RotateEncryptionKeysPhase string

const (
	RotateEncryptionKeysPhasePrepare              RotateEncryptionKeysPhase = "Prepare"
	RotateEncryptionKeysPhasePostPrepareRestart   RotateEncryptionKeysPhase = "PostPrepareRestartNodes"
	RotateEncryptionKeysPhaseRotate               RotateEncryptionKeysPhase = "Rotate"
	RotateEncryptionKeysPhasePostRotateRestart    RotateEncryptionKeysPhase = "PostRotateRestartNodes"
	RotateEncryptionKeysPhaseReencrypt            RotateEncryptionKeysPhase = "Reencrypt"
	RotateEncryptionKeysPhasePostReencryptRestart RotateEncryptionKeysPhase = "PostReencryptRestart"
	RotateEncryptionKeysPhaseDone                 RotateEncryptionKeysPhase = "Done"
	RotateEncryptionKeysPhaseFailed               RotateEncryptionKeysPhase = "Failed"
)

type RKEControlPlaneStatus struct {
	Conditions                    []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready                         bool                                `json:"ready,omitempty"`
	ObservedGeneration            int64                               `json:"observedGeneration"`
	CertificateRotationGeneration int64                               `json:"certificateRotationGeneration"`
	RotateEncryptionKeys          *RotateEncryptionKeys               `json:"rotateEncryptionKeys,omitempty"`
	RotateEncryptionKeysPhase     RotateEncryptionKeysPhase           `json:"rotateEncryptionKeysPhase,omitempty"`
	ETCDSnapshotRestore           *ETCDSnapshotRestore                `json:"etcdSnapshotRestore,omitempty"`
	ETCDSnapshotRestorePhase      ETCDSnapshotPhase                   `json:"etcdSnapshotRestorePhase,omitempty"`
	ETCDSnapshotCreate            *ETCDSnapshotCreate                 `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotCreatePhase       ETCDSnapshotPhase                   `json:"etcdSnapshotCreatePhase,omitempty"`
	ConfigGeneration              int64                               `json:"configGeneration,omitempty"`
	Initialized                   bool                                `json:"initialized,omitempty"`
	AgentConnected                bool                                `json:"agentConnected,omitempty"`
}
