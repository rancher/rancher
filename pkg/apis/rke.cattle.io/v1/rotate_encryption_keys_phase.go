package v1

type RotateEncryptionKeysPhase string

const (
	// RotateEncryptionKeysPhasePrepare is the state assigned to the RKEControlPlane when the encryption key rotation operation is in the Prepare phase.
	// During this phase, the "secrets-encrypt prepare" subcommand is executed on the elected controlplane leader.
	RotateEncryptionKeysPhasePrepare = RotateEncryptionKeysPhase("Prepare")

	// RotateEncryptionKeysPhasePostPrepareRestart is the state assigned to the RKEControlPlane when the encryption key rotation operation is in the Restart phase post executing the prepare command.
	RotateEncryptionKeysPhasePostPrepareRestart = RotateEncryptionKeysPhase("PostPrepareRestart")

	// RotateEncryptionKeysPhaseRotate is the state assigned to the RKEControlPlane when the encryption key rotation operation is in the Rotate phase.
	//During this phase, the "secrets-encrypt rotate" subcommand is executed on the elected controlplane leader.
	RotateEncryptionKeysPhaseRotate = RotateEncryptionKeysPhase("Rotate")

	// RotateEncryptionKeysPhasePostRotateRestart is the state assigned to the RKEControlPlane when the encryption key rotation operation is in the Restart phase post executing the rotate command.
	RotateEncryptionKeysPhasePostRotateRestart = RotateEncryptionKeysPhase("PostRotateRestart")

	// RotateEncryptionKeysPhaseReencrypt is the state assigned to the RKEControlPlane when the encryption key rotation operation is in the Reencrypt phase.
	// During this phase, the "secrets-encrypt reencrypt" subcommand is executed on the elected controlplane leader.
	RotateEncryptionKeysPhaseReencrypt = RotateEncryptionKeysPhase("Reencrypt")

	// RotateEncryptionKeysPhasePostReencryptRestart is the state assigned to the RKEControlPlane when the encryption key rotation operation is in the Restart phase post executing the reencrypt command.
	RotateEncryptionKeysPhasePostReencryptRestart = RotateEncryptionKeysPhase("PostReencryptRestart")

	// RotateEncryptionKeysPhaseDone is the state assigned to the RKEControlPlane upon successful completion of the encryption key rotation operation.
	RotateEncryptionKeysPhaseDone = RotateEncryptionKeysPhase("Done")

	// RotateEncryptionKeysPhaseFailed is the state assigned to the RKEControlPlane upon failure of the encryption key rotation operation.
	RotateEncryptionKeysPhaseFailed = RotateEncryptionKeysPhase("Failed")
)
