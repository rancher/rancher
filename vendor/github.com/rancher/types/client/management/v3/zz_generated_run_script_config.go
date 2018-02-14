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
	Command     string   `json:"command,omitempty"`
	Entrypoint  string   `json:"entrypoint,omitempty"`
	Env         []string `json:"env,omitempty"`
	Image       string   `json:"image,omitempty"`
	IsShell     bool     `json:"isShell,omitempty"`
	ShellScript string   `json:"shellScript,omitempty"`
}
