package client

const (
	ClusterTrustBundleProjectionType               = "clusterTrustBundleProjection"
	ClusterTrustBundleProjectionFieldLabelSelector = "labelSelector"
	ClusterTrustBundleProjectionFieldName          = "name"
	ClusterTrustBundleProjectionFieldOptional      = "optional"
	ClusterTrustBundleProjectionFieldPath          = "path"
	ClusterTrustBundleProjectionFieldSignerName    = "signerName"
)

type ClusterTrustBundleProjection struct {
	LabelSelector *LabelSelector `json:"labelSelector,omitempty" yaml:"labelSelector,omitempty"`
	Name          string         `json:"name,omitempty" yaml:"name,omitempty"`
	Optional      *bool          `json:"optional,omitempty" yaml:"optional,omitempty"`
	Path          string         `json:"path,omitempty" yaml:"path,omitempty"`
	SignerName    string         `json:"signerName,omitempty" yaml:"signerName,omitempty"`
}
