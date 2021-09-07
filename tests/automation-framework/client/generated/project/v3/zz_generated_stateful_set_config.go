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
	Partition            *int64                  `json:"partition,omitempty" yaml:"partition,omitempty"`
	PodManagementPolicy  string                  `json:"podManagementPolicy,omitempty" yaml:"podManagementPolicy,omitempty"`
	RevisionHistoryLimit *int64                  `json:"revisionHistoryLimit,omitempty" yaml:"revisionHistoryLimit,omitempty"`
	ServiceName          string                  `json:"serviceName,omitempty" yaml:"serviceName,omitempty"`
	Strategy             string                  `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	VolumeClaimTemplates []PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty" yaml:"volumeClaimTemplates,omitempty"`
}
