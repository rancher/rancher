package manager

import (
	"fmt"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	v3 "github.com/rancher/rancher/pkg/generated/norman/management.cattle.io/v3"
	"github.com/rancher/rancher/pkg/helm"
)

const invalidURLErrMsg string = "a valid url must consist of an http, https, or git scheme"

type ChartsWithErrors struct {
	Template *helm.ChartVersion
	error    InvalidChartError
}

type InvalidChartError struct {
	Err          string
	TemplateName string
}

func (e *InvalidChartError) Error() string {
	return fmt.Sprintf("failed %v %s", e.TemplateName, e.Err)
}

// Go through each catalog entry and check for errors in values used for template name and labels
// due to restrictions for kubernetes objects. Do not create templates with errored charts.
func (m *Manager) preprocessCatalog(catalog *v3.Catalog, catalogEntries map[string]helm.ChartVersions) ([]ChartsWithErrors, map[string]helm.ChartVersions) {
	var invalidEntries []ChartsWithErrors
	for chart, metadata := range catalogEntries {
		for _, version := range metadata {
			var versionErrors []string
			// get a chart name that matches helm general conventions
			templateName := getValidTemplateName(catalog.Name, chart)
			// check that template name could be used as a kubernetes sub domain
			errorString := validation.IsDNS1123Subdomain(templateName)
			if len(errorString) > 0 {
				versionErrors = append(versionErrors, fmt.Sprintf("template name %s: [%s]", templateName, strings.Join(errorString, ";")))
			}
			// check that template name can be used as a label
			errorString = validation.IsValidLabelValue(templateName)
			if len(errorString) > 0 {
				versionErrors = append(versionErrors, fmt.Sprintf("template name %s: [%s]", templateName, strings.Join(errorString, ";")))
			}
			templateVersionName := getValidTemplateNameWithVersion(templateName, version.Version)
			// validate template version name can be used as a kubernetes sub domain
			errorString = validation.IsDNS1123Subdomain(templateVersionName)
			if len(errorString) > 0 {
				versionErrors = append(versionErrors, fmt.Sprintf("template version name %s: [%s]", templateVersionName, strings.Join(errorString, ";")))
			}
			if version.URLs != nil {
				for _, url := range version.URLs {
					// check to see if url is http, https or git scheme
					if isInvalidVersionURL(url) {
						versionErrors = append(versionErrors, fmt.Sprintf("url %s is invalid. %s", url, invalidURLErrMsg))
					}
				}
			}
			// a chart can have multiple version, so errors are collected per version
			// this allows valid chart versions to be processed for use
			if len(versionErrors) > 0 {
				invalidEntries = append(invalidEntries, ChartsWithErrors{
					Template: version,
					error: InvalidChartError{
						Err:          strings.Join(versionErrors, ";"),
						TemplateName: version.Name,
					},
				})
			}
		}
	}
	// remove invalid entry from catalog entries so that they are not processed into templates
	for _, version := range invalidEntries {
		delete(catalogEntries, version.Template.Name)
	}
	return invalidEntries, catalogEntries
}

func isInvalidVersionURL(versionURL string) bool {
	url, err := url.Parse(versionURL)
	if err != nil {
		return false
	}
	if url.Scheme == "file" || url.Scheme == "local" {
		return true
	}
	return false
}
