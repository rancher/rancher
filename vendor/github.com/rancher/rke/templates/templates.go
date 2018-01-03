package templates

import (
	"bytes"
	"text/template"
)

func CompileTemplateFromMap(tmplt string, configMap map[string]string) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Parse(tmplt))
	if err := t.Execute(out, configMap); err != nil {
		return "", err
	}
	return out.String(), nil
}
