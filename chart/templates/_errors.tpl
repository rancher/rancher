{{ define "tpl.chart.warning" -}}
{{- printf "\n[WARNING] %s\n" . | indent 0 -}}
{{- end -}}

{{ define "tpl.chart.deprecated" -}}
{{ $val := index . 0 -}}
{{ $name := index . 1 -}}
{{ $msg := "" -}}
{{ if ge (len .) 3 -}}
  {{ $msg = index . 2 -}}
{{ end -}}
{{ if $val -}}
{{ printf "[WARNING] Deprecated: %s is deprecated and will be removed in a future release.%s\n" $name $msg | indent 0 }}
{{ end -}}
{{ end -}}

{{ define "tpl.chart.replace" -}}
{{ $val := index . 0 -}}
{{ $old := index . 1 -}}
{{ $new := index . 2 -}}
{{ if $val -}}
{{ printf "[WARNING] Deprecated: %s is deprecated. Please use %s instead.\n" $old $new | indent 0 }}
{{ end -}}
{{ end -}}
