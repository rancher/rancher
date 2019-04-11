package generator

import (
	"bytes"
	"os"
	"text/template"
)

var tmplCache = template.New("template")

func init() {
	tmplCache = tmplCache.Funcs(template.FuncMap{"escapeString": escapeString})
	tmplCache = template.Must(tmplCache.Parse(FilterCustomTagTemplate))
	tmplCache = template.Must(tmplCache.Parse(clusterFilterSyslogTemplate))
	tmplCache = template.Must(tmplCache.Parse(clusterOutputTemplate))
	tmplCache = template.Must(tmplCache.Parse(projectFilterSyslogTemplate))
	tmplCache = template.Must(tmplCache.Parse(projectOutputTemplate))
}

func GenerateConfigFile(outputPath, templateM, tempalteName string, conf map[string]interface{}) error {
	tp, err := tmplCache.Parse(templateM)
	if err != nil {
		return err
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()

	return tp.Execute(output, conf)
}

func GenerateConfig(tempalteName string, conf interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := tmplCache.ExecuteTemplate(buf, tempalteName, conf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
