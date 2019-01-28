package addons

import "github.com/rancher/rke/templates"

func GetCoreDNSManifest(CoreDNSConfig interface{}) (string, error) {

	return templates.CompileTemplateFromMap(templates.CoreDNSTemplate, CoreDNSConfig)
}
