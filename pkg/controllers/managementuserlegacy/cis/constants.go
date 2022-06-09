package cis

const (
	NumberOfRetriesForConfigMapCreate = 3
	NumberOfRetriesForClusterUpdate   = 3
	NumberOfRetriesForClusterGet      = 10
	RetryIntervalInMilliseconds       = 100
	ConfigFileName                    = "config.json"
	CurrentBenchmarkKey               = "current"
	ManualScanPrefix                  = "cis-"
	ScheduledScanPrefix               = "ss-cis-"

	creatorIDAnno = "field.cattle.io/creatorId"
)
