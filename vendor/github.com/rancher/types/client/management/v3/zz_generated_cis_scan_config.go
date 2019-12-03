package client

const (
	CisScanConfigType             = "cisScanConfig"
	CisScanConfigFieldDebugMaster = "debugMaster"
	CisScanConfigFieldDebugWorker = "debugWorker"
	CisScanConfigFieldSkip        = "skip"
)

type CisScanConfig struct {
	DebugMaster bool     `json:"debugMaster,omitempty" yaml:"debugMaster,omitempty"`
	DebugWorker bool     `json:"debugWorker,omitempty" yaml:"debugWorker,omitempty"`
	Skip        []string `json:"skip,omitempty" yaml:"skip,omitempty"`
}
