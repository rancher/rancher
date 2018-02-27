package client

const (
	StatefulSetConfigType                      = "statefulSetConfig"
	StatefulSetConfigFieldPartition            = "partition"
	StatefulSetConfigFieldPodManagementPolicy  = "podManagementPolicy"
	StatefulSetConfigFieldRevisionHistoryLimit = "revisionHistoryLimit"
	StatefulSetConfigFieldServiceName          = "serviceName"
	StatefulSetConfigFieldStrategy             = "strategy"
	StatefulSetConfigFieldVolumeClaimTemplates = "volumeClaimTemplates"
)

type StatefulSetConfig struct {
	Partition            *int64                  `json:"partition,omitempty"`
	PodManagementPolicy  string                  `json:"podManagementPolicy,omitempty"`
	RevisionHistoryLimit *int64                  `json:"revisionHistoryLimit,omitempty"`
	ServiceName          string                  `json:"serviceName,omitempty"`
	Strategy             string                  `json:"strategy,omitempty"`
	VolumeClaimTemplates []PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
}
