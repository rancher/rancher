package client

const (
	RKEMachinePoolType                              = "rkeMachinePool"
	RKEMachinePoolFieldCloudCredentialSecretName    = "cloudCredentialSecretName"
	RKEMachinePoolFieldControlPlaneRole             = "controlPlaneRole"
	RKEMachinePoolFieldDisplayName                  = "displayName"
	RKEMachinePoolFieldDrainBeforeDelete            = "drainBeforeDelete"
	RKEMachinePoolFieldDrainBeforeDeleteTimeout     = "drainBeforeDeleteTimeout"
	RKEMachinePoolFieldEtcdRole                     = "etcdRole"
	RKEMachinePoolFieldLabels                       = "labels"
	RKEMachinePoolFieldMachineDeploymentAnnotations = "machineDeploymentAnnotations"
	RKEMachinePoolFieldMachineDeploymentLabels      = "machineDeploymentLabels"
	RKEMachinePoolFieldMachineOS                    = "machineOS"
	RKEMachinePoolFieldMaxUnhealthy                 = "maxUnhealthy"
	RKEMachinePoolFieldName                         = "name"
	RKEMachinePoolFieldNodeConfig                   = "machineConfigRef"
	RKEMachinePoolFieldNodeStartupTimeout           = "nodeStartupTimeout"
	RKEMachinePoolFieldPaused                       = "paused"
	RKEMachinePoolFieldQuantity                     = "quantity"
	RKEMachinePoolFieldRollingUpdate                = "rollingUpdate"
	RKEMachinePoolFieldTaints                       = "taints"
	RKEMachinePoolFieldUnhealthyNodeTimeout         = "unhealthyNodeTimeout"
	RKEMachinePoolFieldUnhealthyRange               = "unhealthyRange"
	RKEMachinePoolFieldWorkerRole                   = "workerRole"
)

type RKEMachinePool struct {
	CloudCredentialSecretName    string                       `json:"cloudCredentialSecretName,omitempty" yaml:"cloudCredentialSecretName,omitempty"`
	ControlPlaneRole             bool                         `json:"controlPlaneRole,omitempty" yaml:"controlPlaneRole,omitempty"`
	DisplayName                  string                       `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	DrainBeforeDelete            bool                         `json:"drainBeforeDelete,omitempty" yaml:"drainBeforeDelete,omitempty"`
	DrainBeforeDeleteTimeout     string                       `json:"drainBeforeDeleteTimeout,omitempty" yaml:"drainBeforeDeleteTimeout,omitempty"`
	EtcdRole                     bool                         `json:"etcdRole,omitempty" yaml:"etcdRole,omitempty"`
	Labels                       map[string]string            `json:"labels,omitempty" yaml:"labels,omitempty"`
	MachineDeploymentAnnotations map[string]string            `json:"machineDeploymentAnnotations,omitempty" yaml:"machineDeploymentAnnotations,omitempty"`
	MachineDeploymentLabels      map[string]string            `json:"machineDeploymentLabels,omitempty" yaml:"machineDeploymentLabels,omitempty"`
	MachineOS                    string                       `json:"machineOS,omitempty" yaml:"machineOS,omitempty"`
	MaxUnhealthy                 string                       `json:"maxUnhealthy,omitempty" yaml:"maxUnhealthy,omitempty"`
	Name                         string                       `json:"name,omitempty" yaml:"name,omitempty"`
	NodeConfig                   *ObjectReference             `json:"machineConfigRef,omitempty" yaml:"machineConfigRef,omitempty"`
	NodeStartupTimeout           string                       `json:"nodeStartupTimeout,omitempty" yaml:"nodeStartupTimeout,omitempty"`
	Paused                       bool                         `json:"paused,omitempty" yaml:"paused,omitempty"`
	Quantity                     *int64                       `json:"quantity,omitempty" yaml:"quantity,omitempty"`
	RollingUpdate                *RKEMachinePoolRollingUpdate `json:"rollingUpdate,omitempty" yaml:"rollingUpdate,omitempty"`
	Taints                       []Taint                      `json:"taints,omitempty" yaml:"taints,omitempty"`
	UnhealthyNodeTimeout         string                       `json:"unhealthyNodeTimeout,omitempty" yaml:"unhealthyNodeTimeout,omitempty"`
	UnhealthyRange               string                       `json:"unhealthyRange,omitempty" yaml:"unhealthyRange,omitempty"`
	WorkerRole                   bool                         `json:"workerRole,omitempty" yaml:"workerRole,omitempty"`
}
