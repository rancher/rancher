package client

const (
	VolumeProjectionType                     = "volumeProjection"
	VolumeProjectionFieldClusterTrustBundle  = "clusterTrustBundle"
	VolumeProjectionFieldConfigMap           = "configMap"
	VolumeProjectionFieldDownwardAPI         = "downwardAPI"
	VolumeProjectionFieldPodCertificate      = "podCertificate"
	VolumeProjectionFieldSecret              = "secret"
	VolumeProjectionFieldServiceAccountToken = "serviceAccountToken"
)

type VolumeProjection struct {
	ClusterTrustBundle  *ClusterTrustBundleProjection  `json:"clusterTrustBundle,omitempty" yaml:"clusterTrustBundle,omitempty"`
	ConfigMap           *ConfigMapProjection           `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	DownwardAPI         *DownwardAPIProjection         `json:"downwardAPI,omitempty" yaml:"downwardAPI,omitempty"`
	PodCertificate      *PodCertificateProjection      `json:"podCertificate,omitempty" yaml:"podCertificate,omitempty"`
	Secret              *SecretProjection              `json:"secret,omitempty" yaml:"secret,omitempty"`
	ServiceAccountToken *ServiceAccountTokenProjection `json:"serviceAccountToken,omitempty" yaml:"serviceAccountToken,omitempty"`
}
