package client

const (
	AppArmorProfileType                  = "appArmorProfile"
	AppArmorProfileFieldLocalhostProfile = "localhostProfile"
	AppArmorProfileFieldType             = "type"
)

type AppArmorProfile struct {
	LocalhostProfile string `json:"localhostProfile,omitempty" yaml:"localhostProfile,omitempty"`
	Type             string `json:"type,omitempty" yaml:"type,omitempty"`
}
