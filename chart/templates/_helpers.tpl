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
- matchExpressions:
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
Define the chosen value for PSPs. If this value is "", then the user did not set the value. This will
result in psps on <=1.24 and no psps on >=1.25. If the value is true/false, then the user specifically
chose an option, and that option will be used. If it is set otherwise, then we fail so the user can correct
the invalid value.
*/}}

{{- define "rancher.chart_psp_enabled" -}}
{{- if kindIs "bool" .Values.global.cattle.psp.enabled -}}
{{ .Values.global.cattle.psp.enabled }}
{{- else if empty .Values.global.cattle.psp.enabled -}}
  {{- if gt (len (lookup "rbac.authorization.k8s.io/v1" "ClusterRole" "" "")) 0 -}}
    {{- if (.Capabilities.APIVersions.Has "policy/v1beta1/PodSecurityPolicy") -}}
true
    {{- else -}}
false
    {{- end -}}
  {{- else -}}
true
  {{- end -}}
{{- else -}}
{{- fail "Invalid value for .Values.global.cattle.psp.enabled - must be a bool of true, false, or \"\"" -}}
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