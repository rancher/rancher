package client

const (
	RunScriptConfigType             = "runScriptConfig"
	RunScriptConfigFieldImage       = "image"
	RunScriptConfigFieldShellScript = "shellScript"
)

type RunScriptConfig struct {
	Image       string `json:"image,omitempty" yaml:"image,omitempty"`
	ShellScript string `json:"shellScript,omitempty" yaml:"shellScript,omitempty"`
}
