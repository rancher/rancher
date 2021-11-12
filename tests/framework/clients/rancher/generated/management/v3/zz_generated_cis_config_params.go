package client

const (
	CisConfigParamsType                  = "cisConfigParams"
	CisConfigParamsFieldBenchmarkVersion = "benchmarkVersion"
)

type CisConfigParams struct {
	BenchmarkVersion string `json:"benchmarkVersion,omitempty" yaml:"benchmarkVersion,omitempty"`
}
