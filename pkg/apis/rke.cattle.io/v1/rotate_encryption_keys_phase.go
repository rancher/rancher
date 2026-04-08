package v1

type RotateEncryptionKeysPhase string

const (
	// RotateEncryptionKeysPhaseRotate is the state assigned to the RKEControlPlane when the encryption key rotation operation is running on the elected control plane leader.
	// During this phase, the "secrets-encrypt rotate-keys" subcommand is executed on the elected control plane leader and Rancher observes the resulting runtime status.
	RotateEncryptionKeysPhaseRotate = RotateEncryptionKeysPhase("Rotate")

	// RotateEncryptionKeysPhasePostRotateRestart is the state assigned to the RKEControlPlane when Rancher is restarting server nodes after rotate-keys in order to converge high-availability secrets-encrypt status and hashes.
	RotateEncryptionKeysPhasePostRotateRestart = RotateEncryptionKeysPhase("PostRotateRestart")

	// RotateEncryptionKeysPhaseDone is the state assigned to the RKEControlPlane upon successful completion of the encryption key rotation operation.
	RotateEncryptionKeysPhaseDone = RotateEncryptionKeysPhase("Done")

	// RotateEncryptionKeysPhaseFailed is the state assigned to the RKEControlPlane upon failure of the encryption key rotation operation.
	RotateEncryptionKeysPhaseFailed = RotateEncryptionKeysPhase("Failed")
)
