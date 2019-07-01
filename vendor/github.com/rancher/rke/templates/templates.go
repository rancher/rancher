package templates

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/rancher/rke/util"
)

var VersionedTemplate = map[string]map[string]string{
	"calico": map[string]string{
		"v1.15":   CalicoTemplateV115,
		"v1.14":   CalicoTemplateV113,
		"v1.13":   CalicoTemplateV113,
		"default": CalicoTemplateV112,
	},
	"canal": map[string]string{
		"v1.15":   CanalTemplateV115,
		"v1.14":   CanalTemplateV113,
		"v1.13":   CanalTemplateV113,
		"default": CanalTemplateV112,
	},
	"flannel": map[string]string{
		"v1.15":   FlannelTemplateV115,
		"default": FlannelTemplate,
	},
}

func CompileTemplateFromMap(tmplt string, configMap interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Funcs(template.FuncMap{"GetKubednsStubDomains": GetKubednsStubDomains}).Parse(tmplt))
	if err := t.Execute(out, configMap); err != nil {
		return "", err
	}
	return out.String(), nil
}

func GetVersionedTemplates(templateName string, k8sVersion string) string {

	versionedTemplate := VersionedTemplate[templateName]
	if t, ok := versionedTemplate[util.GetTagMajorVersion(k8sVersion)]; ok {
		return t
	}
	return versionedTemplate["default"]
}

func GetKubednsStubDomains(stubDomains map[string][]string) string {
	json, _ := json.Marshal(stubDomains)
	return string(json)
}
