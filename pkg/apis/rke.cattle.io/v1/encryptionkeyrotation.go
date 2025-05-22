package v1

type RotateEncryptionKeysPhase string

const (
	RotateEncryptionKeysPhasePrepare              RotateEncryptionKeysPhase = "Prepare"
	RotateEncryptionKeysPhasePostPrepareRestart   RotateEncryptionKeysPhase = "PostPrepareRestart"
	RotateEncryptionKeysPhaseRotate               RotateEncryptionKeysPhase = "Rotate"
	RotateEncryptionKeysPhasePostRotateRestart    RotateEncryptionKeysPhase = "PostRotateRestart"
	RotateEncryptionKeysPhaseReencrypt            RotateEncryptionKeysPhase = "Reencrypt"
	RotateEncryptionKeysPhasePostReencryptRestart RotateEncryptionKeysPhase = "PostReencryptRestart"
	RotateEncryptionKeysPhaseDone                 RotateEncryptionKeysPhase = "Done"
	RotateEncryptionKeysPhaseFailed               RotateEncryptionKeysPhase = "Failed"
)

type RotateEncryptionKeys struct {
	// Generation defines the current desired generation of encryption key rotation.
	// Setting the generation to a different value than the current generation will trigger a rotation.
	// +optional
	Generation int64 `json:"generation,omitempty"`
}
