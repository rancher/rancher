package uiplugins

import "github.com/rancher/shepherd/pkg/api/steve/catalog/types"

type ExtensionOptions struct {
	ChartName   string
	Version     string
	ReleaseName string
}

// newExtensionsInstall is a private constructor that creates a chart install with given chart values that can be used for chart install action.
func newPluginsInstall(name, version string, chartValues map[string]interface{}) *types.ChartInstall {
	chartInstall := types.ChartInstall{
		ChartName:   name,
		ReleaseName: name,
		Version:     version,
		Values:      nil,
	}

	for k, v := range chartValues {
		chartInstall.Values[k] = v
	}

	return &chartInstall
}


// newPluginUninstallAction is a private constructor that creates a default payload for chart uninstall action with all disabled options.
func newPluginUninstallAction() *types.ChartUninstallAction {
	return &types.ChartUninstallAction{
		DisableHooks: false,
		DryRun:       false,
		KeepHistory:  false,
		Timeout:      nil,
		Description:  "",
	}
}
