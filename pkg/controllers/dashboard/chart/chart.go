package chart

import "github.com/rancher/rancher/pkg/settings"

type Definition struct {
	ReleaseNamespace  string
	ChartName         string
	MinVersionSetting settings.Setting
	Values            func() map[string]interface{}
	Enabled           func() bool
	Uninstall         bool
	RemoveNamespace   bool
}
