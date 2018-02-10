package client

const (
	FlockerVolumeSourceType             = "flockerVolumeSource"
	FlockerVolumeSourceFieldDatasetName = "datasetName"
	FlockerVolumeSourceFieldDatasetUUID = "datasetUUID"
)

type FlockerVolumeSource struct {
	DatasetName string `json:"datasetName,omitempty"`
	DatasetUUID string `json:"datasetUUID,omitempty"`
}
