package generator

import (
	"os"
	"text/template"
)

func GenerateConfigFile(outputPath, templateM, tempalteName string, conf map[string]interface{}) error {
	tp, err := template.New(tempalteName).Parse(templateM)
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
