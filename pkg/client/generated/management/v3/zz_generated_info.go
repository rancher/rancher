package client

const (
	InfoType                       = "info"
	InfoFieldBuildDate             = "buildDate"
	InfoFieldCompiler              = "compiler"
	InfoFieldEmulationMajor        = "emulationMajor"
	InfoFieldEmulationMinor        = "emulationMinor"
	InfoFieldGitCommit             = "gitCommit"
	InfoFieldGitTreeState          = "gitTreeState"
	InfoFieldGitVersion            = "gitVersion"
	InfoFieldGoVersion             = "goVersion"
	InfoFieldMajor                 = "major"
	InfoFieldMinCompatibilityMajor = "minCompatibilityMajor"
	InfoFieldMinCompatibilityMinor = "minCompatibilityMinor"
	InfoFieldMinor                 = "minor"
	InfoFieldPlatform              = "platform"
)

type Info struct {
	BuildDate             string `json:"buildDate,omitempty" yaml:"buildDate,omitempty"`
	Compiler              string `json:"compiler,omitempty" yaml:"compiler,omitempty"`
	EmulationMajor        string `json:"emulationMajor,omitempty" yaml:"emulationMajor,omitempty"`
	EmulationMinor        string `json:"emulationMinor,omitempty" yaml:"emulationMinor,omitempty"`
	GitCommit             string `json:"gitCommit,omitempty" yaml:"gitCommit,omitempty"`
	GitTreeState          string `json:"gitTreeState,omitempty" yaml:"gitTreeState,omitempty"`
	GitVersion            string `json:"gitVersion,omitempty" yaml:"gitVersion,omitempty"`
	GoVersion             string `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Major                 string `json:"major,omitempty" yaml:"major,omitempty"`
	MinCompatibilityMajor string `json:"minCompatibilityMajor,omitempty" yaml:"minCompatibilityMajor,omitempty"`
	MinCompatibilityMinor string `json:"minCompatibilityMinor,omitempty" yaml:"minCompatibilityMinor,omitempty"`
	Minor                 string `json:"minor,omitempty" yaml:"minor,omitempty"`
	Platform              string `json:"platform,omitempty" yaml:"platform,omitempty"`
}
