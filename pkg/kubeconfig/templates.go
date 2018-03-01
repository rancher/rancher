package kubeconfig

import "html/template"

const (
	tokenTemplateText = `apiVersion: v1
kind: Config
clusters:
- name: "{{.ClusterName}}"
  cluster:
    server: "https://{{.Host}}/k8s/clusters/{{.ClusterID}}"
    api-version: v1
    certificate-authority-data: "{{.Cert}}"

users:
- name: "{{.User}}"
  user:
    token: "{{.Token}}"

contexts:
- name: "{{.ClusterName}}"
  context:
    user: "{{.User}}"
    cluster: "{{.ClusterName}}"

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
