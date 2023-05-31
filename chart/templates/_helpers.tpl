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

# Render Values in configurationSnippet
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
Generate the selectorLabels.
*/}}
{{- define "rancher.selectorLabels" -}}
app: {{ template "rancher.fullname" . }}
{{- end }}

# Windows Support

{{/*
Windows cluster will add default taint for linux nodes,
add below linux tolerations to workloads could be scheduled to those linux nodes
*/}}

{{- define "node-tolerations" -}}
- key: "cattle.io/os"
  value: "linux"
  effect: "NoSchedule"
  operator: "Equal"
{{ with .Values.tolerations }}
  {{- toYaml . -}}
{{ end }}
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
Patch the label selector on an object
This template will add a labelSelector using matchLabels to the object referenced at _target if there is no labelSelector specified.
The matchLabels are created with the selectorLabels template.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "rancher.patchLabelSelector" -}}
{{- if not (hasKey ._target "labelSelector") }}
{{- $selectorLabels := (include "rancher.selectorLabels" .) | fromYaml }}
{{- $_ := set ._target "labelSelector" (dict "matchLabels" $selectorLabels) }}
{{- end }}
{{- end }}

{{/*
Patch pod affinity
This template uses the patchLabelSelector template to add a labelSelector to pod affinity objects if there is no labelSelector specified.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "rancher.patchPodAffinity" -}}
{{- if (hasKey ._podAffinity "requiredDuringSchedulingIgnoredDuringExecution") }}
{{- range $term := ._podAffinity.requiredDuringSchedulingIgnoredDuringExecution }}
{{- include "rancher.patchLabelSelector" (merge (dict "_target" $term) $) }}
{{- end }}
{{- end }}
{{- if (hasKey ._podAffinity "preferredDuringSchedulingIgnoredDuringExecution") }}
{{- range $weightedTerm := ._podAffinity.preferredDuringSchedulingIgnoredDuringExecution }}
{{- $_ := set $weightedTerm "podAffinityTerm" (dict) }}
{{- include "rancher.patchLabelSelector" (merge (dict "_target" $weightedTerm.podAffinityTerm) $) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Patch affinity
This template uses patchPodAffinity template to add a labelSelector to podAffinity & podAntiAffinity if one isn't specified.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "rancher.patchAffinity" -}}
{{- if (hasKey .Values.affinity "podAffinity") }}
{{- include "rancher.patchPodAffinity" (merge (dict "_podAffinity" .Values.affinity.podAffinity) .) }}
{{- end }}
{{- if (hasKey .Values.affinity "podAntiAffinity") }}
{{- include "rancher.patchPodAffinity" (merge (dict "_podAffinity" .Values.affinity.podAntiAffinity) .) }}
{{- end }}
{{- end }}

{{/*
Patch topology spread constraints
This template uses the patchLabelSelector template to add a labelSelector to topologySpreadConstraints if one isn't specified.
This works because Helm treats dictionaries as mutable objects and allows passing them by reference.
*/}}
{{- define "rancher.patchTopologySpreadConstraints" -}}
{{- range $constraint := .Values.topologySpreadConstraints }}
{{- include "rancher.patchLabelSelector" (merge (dict "_target" $constraint) $) }}
{{- end }}
{{- end }}
