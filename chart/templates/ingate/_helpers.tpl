{{- define "rancher.hostname" }}
{{- required "hostname is required - please set .Values.hostname to the domain where Rancher will be accessed" .Values.hostname | quote }}
{{- end }}

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

{{/*
Generate parentRefs for HTTPRoute resources
Usage: include "rancher.gateway.parentRefs" (list . "http"|"https")
*/}}
{{- define "rancher.gateway.parentRefs" -}}
{{- $ctx := index . 0 -}}
{{- $listenerType := index . 1 -}}
{{- if $ctx.Values.gateway.createGateway }}
- name: {{ include "rancher.gateway" $ctx }}
  namespace: {{ $ctx.Release.Namespace }}
  sectionName: rancher-{{ $listenerType }}
{{- else }}
{{- $sections := ternary $ctx.Values.gateway.existingGateway.httpsSections $ctx.Values.gateway.existingGateway.httpSections (eq $listenerType "https") }}
{{- range $section := $sections }}
- name: {{ $ctx.Values.gateway.existingGateway.name }}
  {{- if $ctx.Values.gateway.existingGateway.namespace }}
  namespace: {{ $ctx.Values.gateway.existingGateway.namespace }}
  {{- end }}
  sectionName: {{ $section }}
{{- end }}
{{- end }}
{{- end }}
