package charts

const (
	ConfigurationFileKey = "chartUpgrade"
)

type Config struct {
	IsUpgradable bool `json:"isUpgradable" yaml:"isUpgradable"`
}
