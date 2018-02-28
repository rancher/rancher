package client

const (
	RunScriptConfigType             = "runScriptConfig"
	RunScriptConfigFieldCommand     = "command"
	RunScriptConfigFieldEntrypoint  = "entrypoint"
	RunScriptConfigFieldEnv         = "env"
	RunScriptConfigFieldImage       = "image"
	RunScriptConfigFieldIsShell     = "isShell"
	RunScriptConfigFieldShellScript = "shellScript"
)

type RunScriptConfig struct {
	Command     string   `json:"command,omitempty" yaml:"command,omitempty"`
	Entrypoint  string   `json:"entrypoint,omitempty" yaml:"entrypoint,omitempty"`
	Env         []string `json:"env,omitempty" yaml:"env,omitempty"`
	Image       string   `json:"image,omitempty" yaml:"image,omitempty"`
	IsShell     bool     `json:"isShell,omitempty" yaml:"isShell,omitempty"`
	ShellScript string   `json:"shellScript,omitempty" yaml:"shellScript,omitempty"`
}
