package v1

type RotateCertificates struct {
	// Generation defines the current desired generation of certificate rotation.
	// Setting the generation to a different value than the current generation will trigger a rotation.
	// +optional
	Generation int64 `json:"generation,omitempty"`
	// Services is a list of services to rotate certificates for.
	// If the list is empty, all services will be rotated.
	// +optional
	Services []string `json:"services,omitempty"`
}
