package client

const (
	HealthCheckType     = "healthCheck"
	HealthCheckFieldURL = "url"
)

type HealthCheck struct {
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}
