package templates

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/rancher/norman/types/convert"
	"github.com/rancher/rke/metadata"

	"github.com/rancher/rke/util"
)

func CompileTemplateFromMap(tmplt string, configMap interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Funcs(template.FuncMap{"GetKubednsStubDomains": GetKubednsStubDomains}).Parse(tmplt))
	if err := t.Execute(out, configMap); err != nil {
		return "", err
	}
	return out.String(), nil
}

func GetVersionedTemplates(templateName string, data map[string]interface{}, k8sVersion string) string {
	if template, ok := data[templateName]; ok {
		return convert.ToString(template)
	}
	versionedTemplate := metadata.K8sVersionToTemplates[templateName]
	if t, ok := versionedTemplate[util.GetTagMajorVersion(k8sVersion)]; ok {
		return t
	}
	return versionedTemplate["default"]
}

func GetKubednsStubDomains(stubDomains map[string][]string) string {
	json, _ := json.Marshal(stubDomains)
	return string(json)
}

func GetDefaultVersionedTemplate(templateName string, data map[string]interface{}) string {
	if template, ok := data[templateName]; ok {
		return convert.ToString(template)
	}
	return metadata.K8sVersionToTemplates[templateName]["default"]
}
