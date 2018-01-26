package parse

import (
	"github.com/rancher/catalog-controller/utils"
	"github.com/rancher/types/apis/management.cattle.io/v3"
	yaml "gopkg.in/yaml.v2"
)

func TemplateInfo(contents []byte) (v3.Template, error) {
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(contents), &data); err != nil {
		return v3.Template{}, err
	}

	if _, exists := data["projectURL"]; exists {
		data["project_url"] = data["projectURL"]
	}

	if _, exists := data["version"]; exists {
		data["default_version"] = data["version"]
	} else if _, exists := data["defaultVersion"]; exists {
		data["default_version"] = data["defaultVersion"]
	}

	var template v3.Template
	var templateSpec v3.TemplateSpec
	if err := utils.Convert(data, &templateSpec); err != nil {
		return v3.Template{}, err
	}
	template.Spec = templateSpec

	return template, nil
}

func CatalogInfoFromTemplateVersion(contents []byte) (v3.TemplateVersionSpec, error) {
	var template v3.TemplateVersionSpec
	if err := yaml.Unmarshal(contents, &template); err != nil {
		return v3.TemplateVersionSpec{}, err
	}

	return template, nil
}

func CatalogInfoFromRancherCompose(contents []byte) (v3.TemplateVersionSpec, error) {
	cfg, err := utils.CreateConfig(contents)
	if err != nil {
		return v3.TemplateVersionSpec{}, err
	}
	var rawCatalogConfig interface{}

	if cfg.Version == "2" && cfg.Services[".catalog"] != nil {
		rawCatalogConfig = cfg.Services[".catalog"]
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal(contents, &data); err != nil {
		return v3.TemplateVersionSpec{}, err
	}

	if data["catalog"] != nil {
		rawCatalogConfig = data["catalog"]
	} else if data[".catalog"] != nil {
		rawCatalogConfig = data[".catalog"]
	}

	if rawCatalogConfig != nil {
		var template v3.TemplateVersionSpec
		if err := utils.Convert(rawCatalogConfig, &template); err != nil {
			return v3.TemplateVersionSpec{}, err
		}
		return template, nil
	}

	return v3.TemplateVersionSpec{}, nil
}

func CatalogInfoFromCompose(contents []byte) (v3.TemplateVersionSpec, error) {
	contents = []byte(extractCatalogBlock(string(contents)))
	return CatalogInfoFromRancherCompose(contents)
}
