package v3

const (
	// Imported day2ops disable state values persisted in cluster annotations while cleanup progresses.

	ImportedDay2OpsCleaningStateOperations   = "operations"
	ImportedDay2OpsCleaningStateBeacon       = "beacon"
	ImportedDay2OpsCleaningStateMachinePlans = "machine-plans"
	ImportedDay2OpsCleaningStateUninstall    = "uninstall"
	ImportedDay2OpsCleaningStateSUC          = "suc"
)
