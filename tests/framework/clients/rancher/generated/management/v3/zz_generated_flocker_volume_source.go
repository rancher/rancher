package client

const (
	FlockerVolumeSourceType             = "flockerVolumeSource"
	FlockerVolumeSourceFieldDatasetName = "datasetName"
	FlockerVolumeSourceFieldDatasetUUID = "datasetUUID"
)

type FlockerVolumeSource struct {
	DatasetName string `json:"datasetName,omitempty" yaml:"datasetName,omitempty"`
	DatasetUUID string `json:"datasetUUID,omitempty" yaml:"datasetUUID,omitempty"`
}
