{{- define "rancher.hostname" }}
{{- default "rancher.example.com" .Values.hostname | quote }}
{{- end }}

{{/*
Check if Gateway API should be enabled
*/}}
{{- define "rancher.gatewayEnabled" -}}
{{- if eq .Values.networkExposure.type "gateway" -}}
true
{{- else -}}
false
{{- end -}}
{{- end }}

{{- define "rancher.gateway" }}
{{- printf "%s-%s" (include "rancher.fullname" .) "gateway" }}
{{- end }}
