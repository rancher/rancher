package client

const (
	AWSElasticBlockStoreVolumeSourceType           = "awsElasticBlockStoreVolumeSource"
	AWSElasticBlockStoreVolumeSourceFieldFSType    = "fsType"
	AWSElasticBlockStoreVolumeSourceFieldPartition = "partition"
	AWSElasticBlockStoreVolumeSourceFieldReadOnly  = "readOnly"
	AWSElasticBlockStoreVolumeSourceFieldVolumeID  = "volumeID"
)

type AWSElasticBlockStoreVolumeSource struct {
	FSType    string `json:"fsType,omitempty"`
	Partition *int64 `json:"partition,omitempty"`
	ReadOnly  *bool  `json:"readOnly,omitempty"`
	VolumeID  string `json:"volumeID,omitempty"`
}
