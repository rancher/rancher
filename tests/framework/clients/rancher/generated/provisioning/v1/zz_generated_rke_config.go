package client

const (
	RKEConfigType                       = "rkeConfig"
	RKEConfigFieldAdditionalManifest    = "additionalManifest"
	RKEConfigFieldETCD                  = "etcd"
	RKEConfigFieldETCDSnapshotCreate    = "etcdSnapshotCreate"
	RKEConfigFieldETCDSnapshotRestore   = "etcdSnapshotRestore"
	RKEConfigFieldInfrastructureRef     = "infrastructureRef"
	RKEConfigFieldMachineGlobalConfig   = "machineGlobalConfig"
	RKEConfigFieldMachinePools          = "machinePools"
	RKEConfigFieldMachineSelectorConfig = "machineSelectorConfig"
	RKEConfigFieldProvisionGeneration   = "provisionGeneration"
	RKEConfigFieldRegistries            = "registries"
	RKEConfigFieldRotateCertificates    = "rotateCertificates"
	RKEConfigFieldRotateEncryptionKeys  = "rotateEncryptionKeys"
	RKEConfigFieldUpgradeStrategy       = "upgradeStrategy"
)

type RKEConfig struct {
	AdditionalManifest    string                  `json:"additionalManifest,omitempty" yaml:"additionalManifest,omitempty"`
	ETCD                  *ETCD                   `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	ETCDSnapshotCreate    *ETCDSnapshotCreate     `json:"etcdSnapshotCreate,omitempty" yaml:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotRestore   *ETCDSnapshotRestore    `json:"etcdSnapshotRestore,omitempty" yaml:"etcdSnapshotRestore,omitempty"`
	InfrastructureRef     *ObjectReference        `json:"infrastructureRef,omitempty" yaml:"infrastructureRef,omitempty"`
	MachineGlobalConfig   *MachineGlobalConfig    `json:"machineGlobalConfig,omitempty" yaml:"machineGlobalConfig,omitempty"`
	MachinePools          []RKEMachinePool        `json:"machinePools,omitempty" yaml:"machinePools,omitempty"`
	MachineSelectorConfig []RKESystemConfig       `json:"machineSelectorConfig,omitempty" yaml:"machineSelectorConfig,omitempty"`
	ProvisionGeneration   int64                   `json:"provisionGeneration,omitempty" yaml:"provisionGeneration,omitempty"`
	Registries            *Registry               `json:"registries,omitempty" yaml:"registries,omitempty"`
	RotateCertificates    *RotateCertificates     `json:"rotateCertificates,omitempty" yaml:"rotateCertificates,omitempty"`
	RotateEncryptionKeys  *RotateEncryptionKeys   `json:"rotateEncryptionKeys,omitempty" yaml:"rotateEncryptionKeys,omitempty"`
	UpgradeStrategy       *ClusterUpgradeStrategy `json:"upgradeStrategy,omitempty" yaml:"upgradeStrategy,omitempty"`
}
