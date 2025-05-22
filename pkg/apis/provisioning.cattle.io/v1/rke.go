package v1

import (
	rkev1 "github.com/rancher/rancher/pkg/apis/rke.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// RKEMachinePool is the configuration for a RKE2/K3s machine pool within a provisioning cluster.
type RKEMachinePool struct {
	rkev1.RKECommonNodeConfig `json:",inline"`

	// Paused indicates that the machine pool is paused, preventing CAPI controllers from reconciling it.
	// NOTE: this only applies to the corresponding generated machine deployment object, not the generated machines
	// themselves.
	// +optional
	Paused bool `json:"paused,omitempty"`
	// EtcdRole defines whether the machines provisioned by this pool should be etcd nodes.
	// +optional
	EtcdRole bool `json:"etcdRole,omitempty"`
	// ControlPlaneRole defines whether the machines provisioned by this pool should be controlplane nodes.
	// +optional
	ControlPlaneRole bool `json:"controlPlaneRole,omitempty"`
	// WorkerRole defines whether the machines provisioned by this pool should be worker nodes.
	// +optional
	WorkerRole bool `json:"workerRole,omitempty"`
	// DrainBeforeDelete defines whether the machines provisioned by this pool should be drained prior to deletion.
	// +optional
	DrainBeforeDelete bool `json:"drainBeforeDelete,omitempty"`
	// DrainBeforeDeleteTimeout defines the timeout for draining the machines provisioned by this pool before deletion.
	// +optional
	DrainBeforeDeleteTimeout *metav1.Duration `json:"drainBeforeDeleteTimeout,omitempty"`
	// NodeConfig is a reference to a MachineConfig object that will be used to configure the machines provisioned by this pool.
	// The NodeConfig object will, in turn, be used to create a corresponding MachineTemplate object for the generated
	// machine deployment.
	NodeConfig *corev1.ObjectReference `json:"machineConfigRef,omitempty" wrangler:"required"`
	// Name is the internal name of the machine pool.
	// The generated CAPI machine deployment will be a concatenation of the cluster name and the machine pool name
	// which, if over 63 characters is truncated to 54 with a sha256sum appended.
	Name string `json:"name,omitempty" wrangler:"required"`
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// Quantity is the desired number of machines in the machine pool.
	// +optional
	Quantity *int32 `json:"quantity,omitempty"`
	// RollingUpdate is the configuration for the rolling update of the generated machine deployment.
	// +optional
	RollingUpdate *RKEMachinePoolRollingUpdate `json:"rollingUpdate,omitempty"`
	// MachineDeploymentLabels are the labels to add to the generated machine deployment.
	// +optional
	MachineDeploymentLabels map[string]string `json:"machineDeploymentLabels,omitempty"`
	// MachineDeploymentAnnotations are the annotations to add to the generated machine deployment.
	// +optional
	MachineDeploymentAnnotations map[string]string `json:"machineDeploymentAnnotations,omitempty"`
	// NodeStartupTimeout allows setting the maximum time for MachineHealthCheck
	// to consider a Machine unhealthy if a corresponding Node isn't associated
	// through a `Spec.ProviderID` field.
	//
	// The duration set in this field is compared to the greatest of:
	// - Cluster's infrastructure ready condition timestamp (if and when available)
	// - Control Plane's initialized condition timestamp (if and when available)
	// - Machine's infrastructure ready condition timestamp (if and when available)
	// - Machine's metadata creation timestamp
	//
	// Defaults to 10 minutes.
	// If you wish to disable this feature, set the value explicitly to 0.
	// +optional
	NodeStartupTimeout *metav1.Duration `json:"nodeStartupTimeout,omitempty"`
	// UnhealthyNodeTimeout specifies the maximum duration a generated MachineHealthCheck should wait before marking a
	// not ready machine as unhealthy.
	// +optional
	UnhealthyNodeTimeout *metav1.Duration `json:"unhealthyNodeTimeout,omitempty"`
	// MaxUnhealthy specifies the minimum number of unhealthy machines that a MachineHealthCheck can tolerate before
	// remediating unhealthy machines.
	// +optional
	MaxUnhealthy *string `json:"maxUnhealthy,omitempty"`
	// UnhealthyRange specifies the number of unhealthy machines in which a MachineHealthCheck is allowed to remediate.
	// +optional
	UnhealthyRange *string `json:"unhealthyRange,omitempty"`
	// MachineOS is the operating system of the machines provisioned by this pool.
	// This is only used to designate linux versus windows nodes.
	// +kubebuilder:default:=linux
	// +kubebuilder:validation:Enum=linux;windows
	// +optional
	MachineOS string `json:"machineOS,omitempty"`
	// DynamicSchemaSpec is a copy of the dynamic schema object's spec field at the time the machine pool was created.
	// Since rancher-machine based MachineTemplates are not api-versioned, this field is used to drop new fields added
	// to the driver if it has been upgraded since initial provisioning. This allows node drivers to be upgraded
	// without triggering a reconciliation of the provisioning cluster.
	// NOTE: By removing this field, machine pools will be upgraded to the latest version of the node driver.
	// If the spec is the same as it was at creation, the machine pool will not trigger a rollout.
	// +optional
	DynamicSchemaSpec string `json:"dynamicSchemaSpec,omitempty"`
	// HostnameLengthLimit defines the maximum length of the hostname for machines in this pool.
	// For windows nodes, the hostname must be less than 15 characters.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=63
	// +optional
	HostnameLengthLimit int `json:"hostnameLengthLimit,omitempty"`
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
	rkev1.RKEClusterSpecCommon `json:",inline"`

	// ETCDSnapshotCreate is the configuration for the etcd snapshot creation operation.
	// +optional
	ETCDSnapshotCreate *rkev1.ETCDSnapshotCreate `json:"etcdSnapshotCreate,omitempty"`
	// ETCDSnapshotRestore is the configuration for the etcd snapshot restore operation.
	// +optional
	ETCDSnapshotRestore *rkev1.ETCDSnapshotRestore `json:"etcdSnapshotRestore,omitempty"`
	// RotateCertificates is the configuration for the certificate rotation operation.
	// +optional
	RotateCertificates *rkev1.RotateCertificates `json:"rotateCertificates,omitempty"`
	// RotateEncryptionKeys is the configuration for the encryption key rotation operation.
	// +optional
	RotateEncryptionKeys *rkev1.RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty"`

	// MachinePools is a list of machine pools to be created in the provisioning cluster.
	// +optional
	MachinePools []RKEMachinePool `json:"machinePools,omitempty"`
	// MachinePoolDefaults is the default configuration for machine pools.
	// This configuration will be applied to all machine pools unless overridden by the machine pool configuration.
	// +optional
	MachinePoolDefaults RKEMachinePoolDefaults `json:"machinePoolDefaults,omitempty"`
	// InfrastructureRef is a reference to the infrastructure cluster object that is required when provisioning a CAPI cluster.
	// NOTE: in practice this will always be a rkecluster.rke.cattle.io.
	// +optional
	InfrastructureRef *corev1.ObjectReference `json:"infrastructureRef,omitempty"`
}

// RKEMachinePoolDefaults defines the values to set for all machine pools.
// If a value has not been explicitly defined for a machine pool but a default has been set here, then this will be used as a fallback value.
// If a value has been explicitly defined for a machine pool, that value will be used instead.
// NOTE: There is no difference between a zero value and a default value when determining precedence.
type RKEMachinePoolDefaults struct {
	// HostnameLengthLimit defines the maximum length of the hostname for machines in this pool.
	// For windows nodes, the hostname must be less than 15 characters.
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:validation:Maximum=63
	// +optional
	HostnameLengthLimit int `json:"hostnameLengthLimit,omitempty"`
}
