package addons

import "github.com/rancher/rke/templates"

func GetNginxIngressManifest(IngressConfig interface{}, ingressImageVersion string) (string, error) {
	if ingressImageVersion >= "0.32.0" {
		return templates.CompileTemplateFromMap(templates.NginxIngressTemplateV0320Rancher1, IngressConfig)
	}
	return templates.CompileTemplateFromMap(templates.NginxIngressTemplate, IngressConfig)
}
