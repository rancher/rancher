package client

const (
	InfoType              = "info"
	InfoFieldBuildDate    = "buildDate"
	InfoFieldCompiler     = "compiler"
	InfoFieldGitCommit    = "gitCommit"
	InfoFieldGitTreeState = "gitTreeState"
	InfoFieldGitVersion   = "gitVersion"
	InfoFieldGoVersion    = "goVersion"
	InfoFieldMajor        = "major"
	InfoFieldMinor        = "minor"
	InfoFieldPlatform     = "platform"
)

type Info struct {
	BuildDate    string `json:"buildDate,omitempty" yaml:"buildDate,omitempty"`
	Compiler     string `json:"compiler,omitempty" yaml:"compiler,omitempty"`
	GitCommit    string `json:"gitCommit,omitempty" yaml:"gitCommit,omitempty"`
	GitTreeState string `json:"gitTreeState,omitempty" yaml:"gitTreeState,omitempty"`
	GitVersion   string `json:"gitVersion,omitempty" yaml:"gitVersion,omitempty"`
	GoVersion    string `json:"goVersion,omitempty" yaml:"goVersion,omitempty"`
	Major        string `json:"major,omitempty" yaml:"major,omitempty"`
	Minor        string `json:"minor,omitempty" yaml:"minor,omitempty"`
	Platform     string `json:"platform,omitempty" yaml:"platform,omitempty"`
}
