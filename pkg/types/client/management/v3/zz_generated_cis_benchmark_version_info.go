package client

const (
	CisBenchmarkVersionInfoType                      = "cisBenchmarkVersionInfo"
	CisBenchmarkVersionInfoFieldManaged              = "managed"
	CisBenchmarkVersionInfoFieldMinKubernetesVersion = "minKubernetesVersion"
	CisBenchmarkVersionInfoFieldNotApplicableChecks  = "notApplicableChecks"
	CisBenchmarkVersionInfoFieldSkippedChecks        = "skippedChecks"
)

type CisBenchmarkVersionInfo struct {
	Managed              bool              `json:"managed,omitempty" yaml:"managed,omitempty"`
	MinKubernetesVersion string            `json:"minKubernetesVersion,omitempty" yaml:"minKubernetesVersion,omitempty"`
	NotApplicableChecks  map[string]string `json:"notApplicableChecks,omitempty" yaml:"notApplicableChecks,omitempty"`
	SkippedChecks        map[string]string `json:"skippedChecks,omitempty" yaml:"skippedChecks,omitempty"`
}
