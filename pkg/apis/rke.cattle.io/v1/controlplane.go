package v1

import (
	"github.com/rancher/wrangler/pkg/genericcondition"
	corev1 "k8s.io/api/core/v1"
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

type RKEControlPlaneSpec struct {
	RKEClusterSpecCommon

	AgentEnvVars          []corev1.EnvVar     `json:"agentEnvVars,omitempty"`
	ETCDSnapshotCreate    *ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotRestore   *ETCDSnapshot       `json:"etcdSnapshotRestore,omitempty"`
	KubernetesVersion     string              `json:"kubernetesVersion,omitempty"`
	ClusterName           string              `json:"clusterName,omitempty" wrangler:"required"`
	ManagementClusterName string              `json:"managementClusterName,omitempty" wrangler:"required"`
}

type ETCDSnapshotPhase string

var (
	ETCDSnapshotPhaseStarted  ETCDSnapshotPhase = "Started"
	ETCDSnapshotPhaseShutdown ETCDSnapshotPhase = "Shutdown"
	ETCDSnapshotPhaseRestore  ETCDSnapshotPhase = "Restore"
	ETCDSnapshotPhaseFinished ETCDSnapshotPhase = "Finished"
)

type RKEControlPlaneStatus struct {
	Conditions               []genericcondition.GenericCondition `json:"conditions,omitempty"`
	Ready                    bool                                `json:"ready,omitempty"`
	ObservedGeneration       int64                               `json:"observedGeneration"`
	ETCDSnapshotRestore      *ETCDSnapshot                       `json:"etcdSnapshotRestore,omitempty"`
	ETCDSnapshotRestorePhase ETCDSnapshotPhase                   `json:"etcdSnapshotRestorePhase,omitempty"`
	ETCDSnapshotCreate       *ETCDSnapshotCreate                 `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotCreatePhase  ETCDSnapshotPhase                   `json:"etcdSnapshotCreatePhase,omitempty"`
	ConfigGeneration         int64                               `json:"configGeneration,omitempty"`
}
