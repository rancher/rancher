{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "rancher.name" -}}
  {{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "rancher.fullname" -}}
  {{- $name := default .Chart.Name .Values.nameOverride -}}
  {{- if contains $name .Release.Name -}}
    {{- .Release.Name | trunc 63 | trimSuffix "-" -}}
  {{- else -}}
    {{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
  {{- end -}}
{{- end -}}

# Render Values in configurationSnippet
{{- define "configurationSnippet" -}}
  {{- tpl (.Values.ingress.configurationSnippet) . | nindent 6 -}}
{{- end -}}

{{/*
Generate the labels.
*/}}
{{- define "rancher.labels" -}}
app: {{ template "rancher.fullname" . }}
chart: {{ .Chart.Name }}-{{ .Chart.Version }}
heritage: {{ .Release.Service }}
release: {{ .Release.Name }}
{{- end }}

{{- define "system_default_registry" -}}
{{- if .Values.systemDefaultRegistry -}}
  {{- if hasSuffix "/" .Values.systemDefaultRegistry -}}
    {{- printf "%s" .Values.systemDefaultRegistry -}}
  {{- else -}}
    {{- printf "%s/" .Values.systemDefaultRegistry -}}
{{- end -}}
{{- end -}}
{{- end -}}
