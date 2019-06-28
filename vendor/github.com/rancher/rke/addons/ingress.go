package addons

import (
	rkeData "github.com/rancher/kontainer-driver-metadata/rke/templates"
	"github.com/rancher/rke/templates"
)

func GetNginxIngressManifest(IngressConfig interface{}, data map[string]interface{}) (string, error) {
	return templates.CompileTemplateFromMap(templates.GetDefaultVersionedTemplate(rkeData.NginxIngress, data), IngressConfig)
}
