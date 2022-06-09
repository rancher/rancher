package v1

type RotateCertificates struct {
	Generation int64    `json:"generation,omitempty"`
	Services   []string `json:"services,omitempty"`
}
