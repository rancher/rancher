package client

const (
	CisScanConfigType                          = "cisScanConfig"
	CisScanConfigFieldDebugMaster              = "debugMaster"
	CisScanConfigFieldDebugWorker              = "debugWorker"
	CisScanConfigFieldOverrideBenchmarkVersion = "overrideBenchmarkVersion"
	CisScanConfigFieldOverrideSkip             = "overrideSkip"
)

type CisScanConfig struct {
	DebugMaster              bool     `json:"debugMaster,omitempty" yaml:"debugMaster,omitempty"`
	DebugWorker              bool     `json:"debugWorker,omitempty" yaml:"debugWorker,omitempty"`
	OverrideBenchmarkVersion string   `json:"overrideBenchmarkVersion,omitempty" yaml:"overrideBenchmarkVersion,omitempty"`
	OverrideSkip             []string `json:"overrideSkip,omitempty" yaml:"overrideSkip,omitempty"`
}
