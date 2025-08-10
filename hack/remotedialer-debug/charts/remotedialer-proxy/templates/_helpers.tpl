{{- define "system_default_registry" -}}
{{- if .Values.global.cattle.systemDefaultRegistry -}}
{{- printf "%s/" .Values.global.cattle.systemDefaultRegistry -}}
{{- else -}}
{{- "" -}}
{{- end -}}
{{- end -}}

{{/*
API Extension Name - To be used in other variables
*/}}
{{- define "api-extension.name" }}
{{- default "api-extension" .Values.apiExtensionName }}
{{- end}}

{{/*
Namespace to use
*/}}
{{- define "remotedialer-proxy.namespace" -}}
{{- default "cattle-system" .Values.namespaceOverride }}
{{- end }}

{{/*
Expand the name of the chart.
*/}}
{{- define "remotedialer-proxy.name" -}}
{{- default (include "api-extension.name" .) .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "remotedialer-proxy.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "remotedialer-proxy.labels" -}}
helm.sh/chart: {{ include "remotedialer-proxy.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{ include "remotedialer-proxy.selectorLabels" . }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "remotedialer-proxy.selectorLabels" -}}
app.kubernetes.io/name: {{ include "remotedialer-proxy.name" . }}
app.kubernetes.io/instance: {{ include "api-extension.name" . }}
app: {{ include "api-extension.name" . }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "remotedialer-proxy.serviceAccountName" -}}
{{- default (include "api-extension.name" .) .Values.serviceAccount.name }}
{{- end }}

{{/*
Role to use
*/}}
{{- define "remotedialer-proxy.role" -}}
{{- default (include "api-extension.name" .) .Values.roleOverride }}
{{- end }}

{{/*
Role Binding to use
*/}}
{{- define "remotedialer-proxy.rolebinding" -}}
{{- include "api-extension.name" . }}
{{- end }}
