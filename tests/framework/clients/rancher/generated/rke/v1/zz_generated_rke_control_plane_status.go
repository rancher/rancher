package client

const (
	RKEControlPlaneStatusType                               = "rkeControlPlaneStatus"
	RKEControlPlaneStatusFieldAgentConnected                = "agentConnected"
	RKEControlPlaneStatusFieldCertificateRotationGeneration = "certificateRotationGeneration"
	RKEControlPlaneStatusFieldConditions                    = "conditions"
	RKEControlPlaneStatusFieldConfigGeneration              = "configGeneration"
	RKEControlPlaneStatusFieldETCDSnapshotCreate            = "etcdSnapshotCreate"
	RKEControlPlaneStatusFieldETCDSnapshotCreatePhase       = "etcdSnapshotCreatePhase"
	RKEControlPlaneStatusFieldETCDSnapshotRestore           = "etcdSnapshotRestore"
	RKEControlPlaneStatusFieldETCDSnapshotRestorePhase      = "etcdSnapshotRestorePhase"
	RKEControlPlaneStatusFieldInitialized                   = "initialized"
	RKEControlPlaneStatusFieldObservedGeneration            = "observedGeneration"
	RKEControlPlaneStatusFieldReady                         = "ready"
	RKEControlPlaneStatusFieldRotateEncryptionKeys          = "rotateEncryptionKeys"
	RKEControlPlaneStatusFieldRotateEncryptionKeysPhase     = "rotateEncryptionKeysPhase"
)

type RKEControlPlaneStatus struct {
	AgentConnected                bool                  `json:"agentConnected,omitempty" yaml:"agentConnected,omitempty"`
	CertificateRotationGeneration int64                 `json:"certificateRotationGeneration,omitempty" yaml:"certificateRotationGeneration,omitempty"`
	Conditions                    []GenericCondition    `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	ConfigGeneration              int64                 `json:"configGeneration,omitempty" yaml:"configGeneration,omitempty"`
	ETCDSnapshotCreate            *ETCDSnapshotCreate   `json:"etcdSnapshotCreate,omitempty" yaml:"etcdSnapshotCreate,omitempty"`
	ETCDSnapshotCreatePhase       string                `json:"etcdSnapshotCreatePhase,omitempty" yaml:"etcdSnapshotCreatePhase,omitempty"`
	ETCDSnapshotRestore           *ETCDSnapshotRestore  `json:"etcdSnapshotRestore,omitempty" yaml:"etcdSnapshotRestore,omitempty"`
	ETCDSnapshotRestorePhase      string                `json:"etcdSnapshotRestorePhase,omitempty" yaml:"etcdSnapshotRestorePhase,omitempty"`
	Initialized                   bool                  `json:"initialized,omitempty" yaml:"initialized,omitempty"`
	ObservedGeneration            int64                 `json:"observedGeneration,omitempty" yaml:"observedGeneration,omitempty"`
	Ready                         bool                  `json:"ready,omitempty" yaml:"ready,omitempty"`
	RotateEncryptionKeys          *RotateEncryptionKeys `json:"rotateEncryptionKeys,omitempty" yaml:"rotateEncryptionKeys,omitempty"`
	RotateEncryptionKeysPhase     string                `json:"rotateEncryptionKeysPhase,omitempty" yaml:"rotateEncryptionKeysPhase,omitempty"`
}
