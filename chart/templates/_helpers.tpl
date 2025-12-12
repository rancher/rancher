{{/* vim: set filetype=mustache: */}}

{{ define "tpl.url.ensureTrailingSlash" -}}
{{ $url := . | trimSuffix "/" -}}
{{ printf "%s/" $url }}
{{- end -}}

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
Prepare the Rancher Image value w/ new fields as opt-in for now.
*/}}
{{ define "rancher.image" -}}
{{ if .Values.rancherImage -}}
{{ .Values.rancherImage -}}
{{ else -}}
{{ printf "%s%s" (include "defaultOrOverrideRegistry" (list . (default "" .Values.image.registry))) (include "rancher.imageRepo" .) -}}
{{ end -}}
{{ end -}}

{{/*
Prepare the Rancher Image repo value w/ new fields as opt-in for now.
*/}}
{{ define "rancher.imageRepo" -}}
{{ default "rancher/rancher" .Values.image.repository -}}
{{ end -}}


{{/*
Prepare the Rancher Image Tag value w/ new fields as opt-in for now.
*/}}
{{ define "rancher.imageTag" -}}
{{ default .Chart.AppVersion (default .Values.image.tag (default "" .Values.rancherImageTag)) -}}
{{ end -}}

{{/*
Prepare the Rancher Image Pull Policy value w/ new fields as opt-in for now.
*/}}
{{ define "rancher.imagePullPolicy" -}}
{{ default "IfNotPresent" (default .Values.image.pullPolicy (default "" .Values.rancherImagePullPolicy)) -}}
{{ end -}}

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

{{ define "system_default_registry" -}}
{{ if .Values.systemDefaultRegistry -}}
{{ include "tpl.url.ensureTrailingSlash" .Values.systemDefaultRegistry }}
{{- end -}}
{{ end -}}

{{ define "defaultOrOverrideRegistry" -}}
{{ $rootContext := index . 0 -}}
{{ $inputRegistry := index . 1 | default "" -}}
{{ if ne $inputRegistry "" -}}
{{ $inputRegistry = (include "tpl.url.ensureTrailingSlash" $inputRegistry) -}}
{{ end -}}
{{ $systemDefault := include "system_default_registry" $rootContext | default "" -}}
{{ coalesce $inputRegistry $systemDefault "" }}
{{- end -}}

{{/*
    Select correct auditLog image
*/}}
{{ define "auditLog.image" -}}
  {{ if .Values.busyboxImage -}}
    {{ .Values.busyboxImage -}}
  {{ else -}}
    {{- .Values.auditLog.image.repository -}}:{{- .Values.auditLog.image.tag -}}
  {{ end -}}
{{ end -}}

{{- define "rancher.certmanager.notes" -}}
{{- if .Values.ingress.tls.source | eq "rancher" -}}
{{- $requiredVersion := "1.15.0" -}}
{{- $requiredCRD := "certificates.cert-manager.io" -}}
{{- $crdVersion := "v1" -}}

{{- $crd := (lookup "apiextensions.k8s.io/v1" "CustomResourceDefinition" "" $requiredCRD) -}}

{{- if not $crd -}}
{{- $msg := printf "Cert-manager dependency check failed. CRD '%s' not found. Please ensure cert-manager (>= %s) is installed. (Note: This is expected in template/dry-run mode)" $requiredCRD $requiredVersion -}}
{{- include "tpl.chart.warning" $msg -}}
{{- else -}}
  {{- $hasV1 := false -}}
  {{- range $crd.spec.versions -}}
    {{- if and (eq .name $crdVersion) .served -}}
      {{- $hasV1 = true -}}
    {{- end -}}
  {{- end -}}

  {{- if not $hasV1 -}}
    {{- $msg := printf "Cert-manager CRD '%s' found, but it does not support the required API version '%s'. This likely indicates an old cert-manager version. Minimum required version is %s." $requiredCRD $crdVersion $requiredVersion -}}
    {{- include "tpl.chart.warning" $msg -}}
  {{- end -}}
{{- end -}}

{{- $userVersion := .Values.certmanager.version | default "" -}}
{{- if $userVersion -}}
  {{- /* Only execute if version is actually provided and non-empty */ -}}
  {{- if not (semverCompare ">= 0.0.0" $userVersion) -}}
    {{- /* Invalid semver - this will catch parse errors */ -}}
    {{- include "tpl.chart.warning" (printf "Value 'certmanager.version' (%s) is not a valid Semantic Version. Must be >= %s." $userVersion $requiredVersion) -}}
  {{- else if not (semverCompare (printf ">= %s" $requiredVersion) $userVersion) -}}
    {{- /* Valid semver but too old */ -}}
    {{- $msg := printf "The user-provided cert-manager version (%s) is too old. Minimum required version is %s." $userVersion $requiredVersion -}}
    {{- include "tpl.chart.warning" $msg -}}
  {{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}