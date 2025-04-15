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
{{- if .Token }}
    token: "{{.Token}}"
{{ else }}
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
        - token
        - --server={{.Host}}
        - --user={{.User}}
{{- if .EndpointEnabled }}
        - --cluster={{.ClusterID}}
{{- end }}
      command: rancher
{{- end }}

contexts:
{{- range .Nodes}}
- name: "{{.ClusterName}}"
  context:
    user: "{{.User}}"
    cluster: "{{.ClusterName}}"
{{- end}}

current-context: "{{.ClusterName}}"
`

	multiClusterTemplateText = `apiVersion: v1
kind: Config
clusters:
{{- range .Clusters}}
- name: "{{.Name}}"
  cluster:
    server: "{{.Server}}"
{{- if ne .Cert "" }}
    certificate-authority-data: "{{.Cert}}"
{{- end }}
{{- end}}

users:
{{- range .Users}}
- name: "{{.Name}}"
  user:
{{- if .Token }}
    token: "{{.Token}}"
{{ else }}
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      args:
        - token
        - --server={{.Host}}
        - --user={{.Name}}
{{- if ne .ClusterID "" }}
        - --cluster={{.ClusterID}}
{{- end }}
      command: rancher
{{- end }}
{{- end }}

contexts:
{{- range .Contexts}}
- name: "{{.Name}}"
  context:
    user: "{{.User}}"
    cluster: "{{.Cluster}}"
{{- end}}

current-context: "{{.CurrentContext}}"
`
)

var (
	tokenTemplate        = template.Must(template.New("tokenTemplate").Parse(tokenTemplateText))
	multiClusterTemplate = template.Must(template.New("multiClusterTemplate").Parse(multiClusterTemplateText))
)
