package client

const (
	RKEControlPlaneSpecType                          = "rkeControlPlaneSpec"
	RKEControlPlaneSpecFieldAdditionalManifest       = "additionalManifest"
	RKEControlPlaneSpecFieldAgentEnvVars             = "agentEnvVars"
	RKEControlPlaneSpecFieldChartValues              = "chartValues"
	RKEControlPlaneSpecFieldClusterName              = "clusterName"
	RKEControlPlaneSpecFieldETCD                     = "etcd"
	RKEControlPlaneSpecFieldETCDSnapshotCreate       = "etcdSnapshotCreate"
	RKEControlPlaneSpecFieldETCDSnapshotRestore      = "etcdSnapshotRestore"
	RKEControlPlaneSpecFieldKubernetesVersion        = "kubernetesVersion"
	RKEControlPlaneSpecFieldLocalClusterAuthEndpoint = "localClusterAuthEndpoint"
	RKEControlPlaneSpecFieldMachineGlobalConfig      = "machineGlobalConfig"
	RKEControlPlaneSpecFieldMachineSelectorConfig    = "machineSelectorConfig"
	RKEControlPlaneSpecFieldManagementClusterName    = "managementClusterName"
	RKEControlPlaneSpecFieldProvisionGeneration      = "provisionGeneration"
	RKEControlPlaneSpecFieldRegistries               = "registries"
	RKEControlPlaneSpecFieldRotateCertificates       = "rotateCertificates"
	RKEControlPlaneSpecFieldRotateEncryptionKeys     = "rotateEncryptionKeys"
	RKEControlPlaneSpecFieldUnmanagedConfig          = "unmanagedConfig"
	RKEControlPlaneSpecFieldUpgradeStrategy          = "upgradeStrategy"
)

type RKEControlPlaneSpec struct {
	AdditionalManifest       string                    `json:"additionalManifest,omitempty" yaml:"additionalManifest,omitempty"`
	AgentEnvVars             []EnvVar                  `json:"agentEnvVars,omitempty" yaml:"agentEnvVars,omitempty"`
	ChartValues              *GenericMap               `json:"chartValues,omitempty" yaml:"chartValues,omitempty"`
	ClusterName              string                    `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	ETCD                     *ETCD                     `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	ETCDSnapshotCreate       *ETCDSnapshotCreate       `json:"etcdSnapshotCreate,omitempty" yaml:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotRestore      *ETCDSnapshotRestore      `json:"etcdSnapshotRestore,omitempty" yaml:"etcdSnapshotRestore,omitempty"`
	KubernetesVersion        string                    `json:"kubernetesVersion,omitempty" yaml:"kubernetesVersion,omitempty"`
	LocalClusterAuthEndpoint *LocalClusterAuthEndpoint `json:"localClusterAuthEndpoint,omitempty" yaml:"localClusterAuthEndpoint,omitempty"`
	MachineGlobalConfig      *GenericMap               `json:"machineGlobalConfig,omitempty" yaml:"machineGlobalConfig,omitempty"`
	MachineSelectorConfig    []RKESystemConfig         `json:"machineSelectorConfig,omitempty" yaml:"machineSelectorConfig,omitempty"`
	ManagementClusterName    string                    `json:"managementClusterName,omitempty" yaml:"managementClusterName,omitempty"`
	ProvisionGeneration      int64                     `json:"provisionGeneration,omitempty" yaml:"provisionGeneration,omitempty"`
	Registries               *Registry                 `json:"registries,omitempty" yaml:"registries,omitempty"`
	RotateCertificates       *RotateCertificates       `json:"rotateCertificates,omitempty" yaml:"rotateCertificates,omitempty"`
	RotateEncryptionKeys     *RotateEncryptionKeys     `json:"rotateEncryptionKeys,omitempty" yaml:"rotateEncryptionKeys,omitempty"`
	UnmanagedConfig          bool                      `json:"unmanagedConfig,omitempty" yaml:"unmanagedConfig,omitempty"`
	UpgradeStrategy          *ClusterUpgradeStrategy   `json:"upgradeStrategy,omitempty" yaml:"upgradeStrategy,omitempty"`
}
