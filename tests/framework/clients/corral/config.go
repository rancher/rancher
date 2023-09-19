package corral

import (
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

// The json/yaml config key for the corral package to be build ..
const (
	corralPackageConfigConfigurationFileKey = "corralPackages"
	corralConfigConfigurationFileKey        = "corralConfigs"
)

// Configs is a struct that for necessary corral config environment variables to build a corral
type Configs struct {
	CorralConfigVars map[string]string `json:"corralConfigVars" yaml:"corralConfigVars"`
	CorralConfigUser string            `json:"corralConfigUser" yaml:"corralConfigUser" default:"jenkauto"`
	CorralSSHPath    string            `json:"corralSSHPath" yaml:"corralSSHPath" default:"/root/.ssh/public.pub"`
}

// Packages is a struct that has the path to the packages
type Packages struct {
	CorralPackageImages map[string]string `json:"corralPackageImages" yaml:"corralPackageImages"`
	HasCleanup          bool              `json:"hasCleanup" yaml:"hasCleanup" default:"true"`
	HasDebug            bool              `json:"hasDebug" yaml:"hasDebug" default:"false"`
	HasCustomRepo       string            `json:"hasCustomRepo" yaml:"hasCustomRepo"`
}

// PackagesConfig is a function that reads in the corral package object from the config file
func PackagesConfig() *Packages {
	var corralPackages Packages
	config.LoadConfig(corralPackageConfigConfigurationFileKey, &corralPackages)
	return &corralPackages
}

// Configurations is a function that reads in the corral config vars from the config file
func Configurations() *Configs {
	var corralConfigs Configs
	config.LoadConfig(corralConfigConfigurationFileKey, &corralConfigs)
	return &corralConfigs
}
