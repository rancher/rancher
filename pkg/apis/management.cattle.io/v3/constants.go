package v3

import (
	cond "github.com/rancher/norman/condition"
)

const (
	// transition type

	Created      cond.Cond = "Created"
	RunCompleted cond.Cond = "RunCompleted"

	// done type

	Completed cond.Cond = "Completed"
	Ready     cond.Cond = "Ready"

	// error type

	Failed cond.Cond = "Failed"

	// generic type
	// these will not trigger any state change on the object

	Alerted cond.Cond = "Alerted"

	// Imported day2ops disable state values persisted in cluster annotations while cleanup progresses.
	ImportedDay2OpsCleaningStateOperations   = "operations"
	ImportedDay2OpsCleaningStateBeacon       = "beacon"
	ImportedDay2OpsCleaningStateMachinePlans = "machine-plans"
	ImportedDay2OpsCleaningStateUninstall    = "uninstall"
	ImportedDay2OpsCleaningStateSUC          = "suc"
)
