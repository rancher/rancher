package qaseinput

const (
	ConfigurationFileKey = "qaseInput"
)

type Config struct {
	LocalQaseReporting bool `yaml:"localQaseReporting" json:"localQaseReporting" default:"false"`
}
