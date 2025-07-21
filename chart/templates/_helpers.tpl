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

{{/*
Create a default fully qualified chart name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "rancher.chartname" -}}
  {{- printf "%s-%s" .Chart.Name .Chart.Version | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Render Values in configurationSnippet
*/}}
{{- define "configurationSnippet" -}}
  {{- tpl (.Values.ingress.configurationSnippet) . | nindent 6 -}}
{{- end -}}

{{/*
Generate the labels.
*/}}
{{- define "rancher.labels" -}}
app: {{ template "rancher.fullname" . }}
chart: {{ template "rancher.chartname" . }}
heritage: {{ .Release.Service }}
release: {{ .Release.Name }}
{{- end }}

{{/*
Generate the labels for pre-upgrade-hook.
*/}}
{{- define "rancher.preupgradelabels" -}}
app: {{ template "rancher.fullname" . }}-pre-upgrade
chart: {{ template "rancher.chartname" . }}
heritage: {{ .Release.Service }}
release: {{ .Release.Name }}
{{- end }}

{{/*
Generate the Kubernetes recommended common labels.

Usage:
  include "rancher.commonLabels" (dict "context" . "component" "xyz" "partOf" "abc")
*/}}
{{- define "rancher.commonLabels" -}}
{{- $ctx := .context }}
app.kubernetes.io/name: {{ $ctx.Chart.Name | quote }}
app.kubernetes.io/instance: {{ $ctx.Release.Name | quote }}
app.kubernetes.io/version: {{ $ctx.Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ $ctx.Release.Service | quote }}
{{- with .component }}
app.kubernetes.io/component: {{ . | quote }}
{{- end }}
{{- with .partOf }}
app.kubernetes.io/part-of: {{ . | quote }}
{{- end }}
{{- end }}

# Windows Support

{{/*
Windows cluster will add default taint for linux nodes,
add below linux tolerations to workloads could be scheduled to those linux nodes
*/}}

{{- define "linux-node-tolerations" -}}
- key: "cattle.io/os"
  value: "linux"
  effect: "NoSchedule"
  operator: "Equal"
{{- end -}}

{{- define "linux-node-selector-terms" -}}
{{- $key := "kubernetes.io/os" -}}
- key: {{ $key }}
  operator: NotIn
  values:
  - windows
{{- end -}}

{{- define "system_default_registry" -}}
{{- if .Values.systemDefaultRegistry -}}
  {{- if hasSuffix "/" .Values.systemDefaultRegistry -}}
    {{- printf "%s" .Values.systemDefaultRegistry -}}
  {{- else -}}
    {{- printf "%s/" .Values.systemDefaultRegistry -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
    Select correct auditLog image
*/}}
{{- define "auditLog_image" -}}
  {{- if .Values.busyboxImage }}
    {{- .Values.busyboxImage}}
  {{- else }}
    {{- .Values.auditLog.image.repository -}}:{{- .Values.auditLog.image.tag -}}
  {{- end }}
{{- end -}}

{{/*
    Determine the registration mode, defaulting to online if not specified
*/}}
{{ define "registration.mode" -}}
{{ default "online" .Values.registration.mode | quote }}
{{- end -}}