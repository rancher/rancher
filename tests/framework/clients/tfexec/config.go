package tfexec

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

const (
	ConfigurationFileKey = "tfexecInput"
)

type PlanOptions struct {
	OutDir string `json:"outDir" yaml:"outDir"`
}

type Config struct {
	WorkspaceName string       `json:"workspaceName" yaml:"workspaceName"`
	WorkingDir    string       `json:"workingDir" yaml:"workingDir"`
	ExecPath      string       `json:"execPath" yaml:"execPath"`
	VarFilePath   string       `json:"varFilePath" yaml:"varFilePath"`
	PlanFilePath  string       `json:"planFilePath" yaml:"planFilePath"`
	PlanOpts      *PlanOptions `json:"planOpts" yaml:"planOpts"`
}

func TerraformConfig() *Config {
	var tfConfig Config
	config.LoadConfig(ConfigurationFileKey, &tfConfig)
	return &tfConfig
}
