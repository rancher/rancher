{{/*
Check if TLS for rancher terminates external or local to k8s cluster
*/}}
{{- define "rancher.useExternalTls" -}}
{{- if eq .Values.tls "external" -}}
true
{{- else -}}
false
{{- end -}}
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

{{/*
Check if Ingress should be enabled
*/}}
{{- define "rancher.ingressEnabled" -}}
{{- if and (eq .Values.networkExposure.type "ingress") .Values.ingress.enabled -}}
true
{{- else -}}
false
{{- end -}}
{{- end }}

{{- define "rancher.gateway" }}
{{- printf "%s-%s" (include "rancher.fullname" .) "gateway" }}
{{- end }}
