package kubeconfig

import "html/template"

const (
	tokenTemplateText = `apiVersion: v1
kind: Config
clusters:
{{- range .Nodes}}
- name: "{{.ClusterName}}"
  cluster:
    server: "{{.Server}}"
{{- if ne .Cert "" }}
    certificate-authority-data: "{{.Cert}}"
{{- end }}
{{- end}}

users:
- name: "{{.User}}"
  user:
    token: "{{.Token}}"

contexts:
{{- range .Nodes}}
- name: "{{.ClusterName}}"
  context:
    user: "{{.User}}"
    cluster: "{{.ClusterName}}"
{{- end}}

current-context: "{{.ClusterName}}"
`

	basicTemplateText = `apiVersion: v1
kind: Config
clusters:
- name: "{{.ClusterName}}"
  cluster:
    server: "https://{{.Host}}"
    api-version: v1

users:
- name: "{{.User}}"
  user:
    username: "{{.Username}}"
    password: "{{.Password}}"

contexts:
- name: "{{.ClusterName}}"
  context:
    user: "{{.User}}"
    cluster: "{{.ClusterName}}"

current-context: "{{.ClusterName}}"
`
)

var (
	basicTemplate = template.Must(template.New("basicTemplate").Parse(basicTemplateText))
	tokenTemplate = template.Must(template.New("tokenTemplate").Parse(tokenTemplateText))
)
