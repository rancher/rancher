package client

const (
	NetworkConfigType         = "networkConfig"
	NetworkConfigFieldOptions = "options"
	NetworkConfigFieldPlugin  = "plugin"
)

type NetworkConfig struct {
	Options map[string]string `json:"options,omitempty"`
	Plugin  string            `json:"plugin,omitempty"`
}
