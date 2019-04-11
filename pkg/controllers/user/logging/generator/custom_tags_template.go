package generator

var FilterCustomTagTemplate = `
{{define "filter-custom-tags"}}
<filter {{ .ContainerLogSourceTag }}.**>
  @type record_transformer
  <record>
    tag ${tag}
    log_type k8s_normal_container 
    {{- range $k, $val := .OutputTags }}
    {{$k}}  {{$val | escapeString}}
    {{end}}
  </record>
</filter>
{{end}}
`
