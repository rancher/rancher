package addons

import "github.com/rancher/rke/templates"

func GetMetricsServerManifest(MetricsServerConfig interface{}) (string, error) {

	return templates.CompileTemplateFromMap(templates.MetricsServerTemplate, MetricsServerConfig)
}
