package v1

// ETCDSnapshotPhase is a representation of the current phase of an etcd snapshot create or restore operation.
type ETCDSnapshotPhase string

const (
	// ETCDSnapshotPhaseStarted is the first state the RKEControlPlane is assigned when beginning an etcd snapshot or restore operation.
	ETCDSnapshotPhaseStarted = ETCDSnapshotPhase("Started")

	// ETCDSnapshotPhaseShutdown is the state assigned to the RKEControlPlane when the etcd restore operation is performing the shutdown of the cluster in order to perform a restore.
	ETCDSnapshotPhaseShutdown = ETCDSnapshotPhase("Shutdown")

	// ETCDSnapshotPhaseRestore is the state assigned to the RKEControlPlane when the etcd restore operation is restoring etcd.
	ETCDSnapshotPhaseRestore = ETCDSnapshotPhase("Restore")

	// ETCDSnapshotPhasePostRestoreCleanup is the state assigned to the RKEControlPlane when the etcd restore operation is removing old pods post-restore.
	ETCDSnapshotPhasePostRestoreCleanup = ETCDSnapshotPhase("PostRestoreCleanup")

	// ETCDSnapshotPhaseRestartCluster is the state assigned to the RKEControlPlane when the etcd snapshot create/restore operation is restarting the cluster.
	ETCDSnapshotPhaseRestartCluster = ETCDSnapshotPhase("RestartCluster")

	// ETCDSnapshotPhaseFinished is the state assigned to the RKEControlPlane upon successful completion of the snapshot create/restore operation.
	ETCDSnapshotPhaseFinished = ETCDSnapshotPhase("Finished")

	// ETCDSnapshotPhaseFailed is the state assigned to the RKEControlPlane upon failure of the etcd snapshot create/restore operation.
	ETCDSnapshotPhaseFailed = ETCDSnapshotPhase("Failed")
)
