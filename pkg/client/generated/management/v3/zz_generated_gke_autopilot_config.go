package client

const (
	GKEAutopilotConfigType         = "gkeAutopilotConfig"
	GKEAutopilotConfigFieldEnabled = "enabled"
)

type GKEAutopilotConfig struct {
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}
