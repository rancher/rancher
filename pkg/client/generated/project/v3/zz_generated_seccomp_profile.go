package client

const (
	SeccompProfileType                  = "seccompProfile"
	SeccompProfileFieldLocalhostProfile = "localhostProfile"
	SeccompProfileFieldType             = "type"
)

type SeccompProfile struct {
	LocalhostProfile string `json:"localhostProfile,omitempty" yaml:"localhostProfile,omitempty"`
	Type             string `json:"type,omitempty" yaml:"type,omitempty"`
}
