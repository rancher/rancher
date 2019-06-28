package addons

import (
	rkeData "github.com/rancher/kontainer-driver-metadata/rke/templates"
	"github.com/rancher/rke/templates"
)

func GetCoreDNSManifest(CoreDNSConfig interface{}, data map[string]interface{}) (string, error) {
	return templates.CompileTemplateFromMap(templates.GetDefaultVersionedTemplate(rkeData.CoreDNS, data), CoreDNSConfig)
}
