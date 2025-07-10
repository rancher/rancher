package v1

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type RKEMachinePool struct {
	rkev1.RKECommonNodeConfig

	Paused                       bool                         `json:"paused,omitempty"`
	EtcdRole                     bool                         `json:"etcdRole,omitempty"`
	ControlPlaneRole             bool                         `json:"controlPlaneRole,omitempty"`
	WorkerRole                   bool                         `json:"workerRole,omitempty"`
	DrainBeforeDelete            bool                         `json:"drainBeforeDelete,omitempty"`
	DrainBeforeDeleteTimeout     *metav1.Duration             `json:"drainBeforeDeleteTimeout,omitempty"`
	NodeConfig                   *corev1.ObjectReference      `json:"machineConfigRef,omitempty" wrangler:"required"`
	Name                         string                       `json:"name,omitempty" wrangler:"required"`
	DisplayName                  string                       `json:"displayName,omitempty"`
	Quantity                     *int32                       `json:"quantity,omitempty"`
	RollingUpdate                *RKEMachinePoolRollingUpdate `json:"rollingUpdate,omitempty"`
	MachineDeploymentLabels      map[string]string            `json:"machineDeploymentLabels,omitempty"`
	MachineDeploymentAnnotations map[string]string            `json:"machineDeploymentAnnotations,omitempty"`
	NodeStartupTimeout           *metav1.Duration             `json:"nodeStartupTimeout,omitempty"`
	UnhealthyNodeTimeout         *metav1.Duration             `json:"unhealthyNodeTimeout,omitempty"`
	MaxUnhealthy                 *string                      `json:"maxUnhealthy,omitempty"`
	UnhealthyRange               *string                      `json:"unhealthyRange,omitempty"`
	MachineOS                    string                       `json:"machineOS,omitempty"`
	DynamicSchemaSpec            string                       `json:"dynamicSchemaSpec,omitempty"`
	HostnameLengthLimit          int                          `json:"hostnameLengthLimit,omitempty"`
}

type RKEMachinePoolRollingUpdate struct {
	// The maximum number of machines that can be unavailable during the update.
	// Value can be an absolute number (ex: 5) or a percentage of desired
	// machines (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	// This can not be 0 if MaxSurge is 0.
	// Defaults to 0.
	// Example: when this is set to 30%, the old MachineSet can be scaled
	// down to 70% of desired machines immediately when the rolling update
	// starts. Once new machines are ready, old MachineSet can be scaled
	// down further, followed by scaling up the new MachineSet, ensuring
	// that the total number of machines available at all times
	// during the update is at least 70% of desired machines.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// The maximum number of machines that can be scheduled above the
	// desired number of machines.
	// Value can be an absolute number (ex: 5) or a percentage of
	// desired machines (ex: 10%).
	// This can not be 0 if MaxUnavailable is 0.
	// Absolute number is calculated from percentage by rounding up.
	// Defaults to 1.
	// Example: when this is set to 30%, the new MachineSet can be scaled
	// up immediately when the rolling update starts, such that the total
	// number of old and new machines do not exceed 130% of desired
	// machines. Once old machines have been killed, new MachineSet can
	// be scaled up further, ensuring that total number of machines running
	// at any time during the update is at most 130% of desired machines.
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// Note: if you add new fields to the RKEConfig, please ensure that you check
// `pkg/controllers/provisioningv2/rke2/provisioningcluster/template.go` file and
// drop the fields when saving a copy of the cluster spec on etcd snapshots, otherwise,
// operations using the new fields will cause unnecessary plan thrashing.

type RKEConfig struct {
	rkev1.ClusterConfiguration

	ETCDSnapshotCreate   *rkev1.ETCDSnapshotCreate   `json:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotRestore  *rkev1.ETCDSnapshotRestore  `json:"etcdSnapshotRestore,omitempty"`
	RotateCertificates   *rkev1.RotateCertificates   `json:"rotateCertificates,omitempty"`
	RotateEncryptionKeys *rkev1.RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	MachinePools        []RKEMachinePool        `json:"machinePools,omitempty"`
	MachinePoolDefaults RKEMachinePoolDefaults  `json:"machinePoolDefaults,omitempty"`
	InfrastructureRef   *corev1.ObjectReference `json:"infrastructureRef,omitempty"`
}

type RKEMachinePoolDefaults struct {
	HostnameLengthLimit int `json:"hostnameLengthLimit,omitempty"`
}
