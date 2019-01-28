package addons

import "github.com/rancher/rke/templates"

func GetKubeDNSManifest(KubeDNSConfig interface{}) (string, error) {

	return templates.CompileTemplateFromMap(templates.KubeDNSTemplate, KubeDNSConfig)
}
