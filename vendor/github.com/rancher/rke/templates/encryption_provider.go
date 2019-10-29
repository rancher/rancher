package templates

const (
	DisabledEncryptionProviderFile = `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- resources:
  - secrets
  providers:
  - identity: {}
  - aescbc:
      keys:
      - name: {{.Name}}
        secret: {{.Secret}}`

	MultiKeyEncryptionProviderFile = `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
- resources:
  - secrets
  providers:
  - aescbc:
      keys:
{{- range $i, $v:= .KeyList}}
      - name: {{ $v.Name}}
        secret: {{ $v.Secret -}}
{{end}}
  - identity: {}`

	CustomEncryptionProviderFile = `apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
{{.CustomConfig}}
`
)
