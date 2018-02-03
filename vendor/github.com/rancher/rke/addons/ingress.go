package addons

import "github.com/rancher/rke/templates"

func GetNginxIngressManifest(IngressConfig interface{}) (string, error) {

	return templates.CompileTemplateFromMap(templates.NginxIngressTemplate, IngressConfig)
}
