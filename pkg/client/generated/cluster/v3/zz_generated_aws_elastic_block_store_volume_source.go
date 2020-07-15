package client

const (
	AWSElasticBlockStoreVolumeSourceType           = "awsElasticBlockStoreVolumeSource"
	AWSElasticBlockStoreVolumeSourceFieldFSType    = "fsType"
	AWSElasticBlockStoreVolumeSourceFieldPartition = "partition"
	AWSElasticBlockStoreVolumeSourceFieldReadOnly  = "readOnly"
	AWSElasticBlockStoreVolumeSourceFieldVolumeID  = "volumeID"
)

type AWSElasticBlockStoreVolumeSource struct {
	FSType    string `json:"fsType,omitempty" yaml:"fsType,omitempty"`
	Partition int64  `json:"partition,omitempty" yaml:"partition,omitempty"`
	ReadOnly  bool   `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	VolumeID  string `json:"volumeID,omitempty" yaml:"volumeID,omitempty"`
}
