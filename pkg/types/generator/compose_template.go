package generator

var composeTemplate = `package compose

import (
	clusterClient "github.com/rancher/rancher/pkg/types/client/cluster/v3"
	managementClient "github.com/rancher/rancher/pkg/types/client/management/v3"
	projectClient "github.com/rancher/rancher/pkg/types/client/project/v3"
)

type Config struct {
	Version string %BACK%yaml:"version,omitempty"%BACK%

	// Management Client
	{{range .managementSchemas}}
    {{- if . | hasPost }}{{.CodeName}}s map[string]managementClient.{{.CodeName}} %BACK%json:"{{.PluralName}},omitempty" yaml:"{{.PluralName}},omitempty"%BACK%
{{end}}{{end}}

	// Cluster Client
	{{range .clusterSchemas}}
	{{- if . | hasGet }}{{.CodeName}}s map[string]clusterClient.{{.CodeName}} %BACK%json:"{{.PluralName}},omitempty" yaml:"{{.PluralName}},omitempty"%BACK%
{{end}}{{end}}

	// Project Client
	{{range .projectSchemas}}
	{{- if . | hasGet }}{{.CodeName}}s map[string]projectClient.{{.CodeName}} %BACK%json:"{{.PluralName}},omitempty" yaml:"{{.PluralName}},omitempty"%BACK%
{{end}}{{end}}}`
