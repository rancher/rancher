{{- define "rancher.hostname" }}
{{- default "rancher.example.com" .Values.hostname | quote }}
{{- end }}