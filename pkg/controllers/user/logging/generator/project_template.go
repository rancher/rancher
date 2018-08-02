package generator

var ProjectTemplate = `{{range $i, $store := .projectTargets -}}
{{ if $store.CurrentTarget }}
<source>
   @type  tail
   path  /var/log/containers/*.log
   pos_file  /fluentd/log/fluentd-project-{{$store.ProjectName}}-logging.pos
   time_format  %Y-%m-%dT%H:%M:%S
   tag  {{$store.ProjectName}}.*
   format  json
   read_from_head  true
</source>

<filter  {{$store.ProjectName}}.**>
   @type  kubernetes_metadata
   merge_json_log  true
   preserve_json_log  true
</filter>

<filter {{$store.ProjectName}}.**>
  @type record_transformer
  enable_ruby  true
  <record>
    tag ${tag}
    namespace ${record["kubernetes"]["namespace_name"]}
    {{range $k, $val := $store.OutputTags -}}
    {{$k}} {{$val}}
    {{end -}}
    projectID {{$store.ProjectName}}
  </record>
</filter>

<filter {{$store.ProjectName}}.**>
  @type grep
  <regexp>
    key namespace
    pattern {{$store.GrepNamespace}}
  </regexp>
</filter>

<filter {{$store.ProjectName}}.**>
  @type record_transformer
  remove_keys namespace
</filter>

{{ if eq $store.CurrentTarget "syslog"}}
{{ if $store.SyslogConfig.Token}}
<filter {{$store.ProjectName}}.** project-custom.{{$store.ProjectName}}.**>
  @type record_transformer
  <record>
    tag ${tag} {{$store.SyslogConfig.Token}}
  </record>
</filter>
{{end -}}
{{end -}}

<match  {{$store.ProjectName}}.** project-custom.{{$store.ProjectName}}.**> 
    {{ if eq $store.CurrentTarget "elasticsearch"}}
    @type elasticsearch
    include_tag_key  true
    {{ if and $store.ElasticsearchConfig.AuthUserName $store.ElasticsearchConfig.AuthPassword}}
    hosts {{$store.WrapElasticsearch.Scheme}}://{{$store.ElasticsearchConfig.AuthUserName}}:{{$store.ElasticsearchConfig.AuthPassword}}@{{$store.WrapElasticsearch.Host}}
    {{else -}}
    hosts {{$store.ElasticsearchConfig.Endpoint}}    
    {{end -}}

    logstash_prefix "{{$store.ElasticsearchConfig.IndexPrefix}}"
    logstash_format true
    logstash_dateformat  {{$store.WrapElasticsearch.DateFormat}}
    type_name  "container_log"

    {{ if eq $store.WrapElasticsearch.Scheme "https"}}
    ssl_verify {{$store.ElasticsearchConfig.SSLVerify}}

    {{ if $store.ElasticsearchConfig.Certificate }}
    ca_file /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_ca.pem
    {{end -}}

    {{ if and $store.ElasticsearchConfig.ClientCert $store.ElasticsearchConfig.ClientKey}}
    client_cert /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-cert.pem
    client_key /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-key.pem
    {{end -}}

    {{ if $store.ElasticsearchConfig.ClientKeyPass}}
    client_key_pass {{$store.ElasticsearchConfig.ClientKeyPass}}
    {{end -}}
    {{end -}}
    {{end -}}

    {{ if eq $store.CurrentTarget "splunk"}}
    @type splunk_hec
    host {{$store.WrapSplunk.Host}}
    port {{$store.WrapSplunk.Port}}
    token {{$store.SplunkConfig.Token}}
    {{ if $store.SplunkConfig.Source}}
    sourcetype {{$store.SplunkConfig.Source}}
    {{end -}}
    {{ if $store.SplunkConfig.Index}}
    default_index {{ $store.SplunkConfig.Index }}
    {{end -}}

    {{ if eq $store.WrapSplunk.Scheme "https"}}
    use_ssl true    
    ssl_verify {{$store.SplunkConfig.SSLVerify}}    

    {{ if $store.SplunkConfig.Certificate }}    
    ca_file /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_ca.pem
    {{end -}}

    {{ if and $store.SplunkConfig.ClientCert $store.SplunkConfig.ClientKey}}    
    client_cert /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-cert.pem
    client_key /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-key.pem
    {{end -}}

    {{ if $store.SplunkConfig.ClientKeyPass}}    
    client_key_pass {{ $store.SplunkConfig.ClientKeyPass }}
    {{end -}}
    {{end -}}
    {{end -}}

    {{ if eq $store.CurrentTarget "kafka"}}
    @type kafka_buffered
    {{ if $store.KafkaConfig.ZookeeperEndpoint }}
    zookeeper {{$store.WrapKafka.Zookeeper}}
    {{else}}
    brokers {{$store.WrapKafka.Brokers}}
    {{end}}
    default_topic {{$store.KafkaConfig.Topic}}
    output_data_type  "json"
    output_include_tag  true
    output_include_time  true
    # get_kafka_client_log  true
    max_send_retries 3
    
    {{ if $store.KafkaConfig.Certificate }}        
    ssl_ca_cert /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_ca.pem
    {{end}}

    {{ if and $store.KafkaConfig.ClientCert $store.KafkaConfig.ClientKey}}        
    ssl_client_cert /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-cert.pem
    ssl_client_cert_key /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-key.pem
    {{ end -}}
    {{end -}}

    {{ if eq $store.CurrentTarget "syslog"}}
    @type remote_syslog
    host {{$store.WrapSyslog.Host}}
    port {{$store.WrapSyslog.Port}}
    severity {{$store.SyslogConfig.Severity}}
    program {{$store.SyslogConfig.Program}}
    protocol {{$store.SyslogConfig.Protocol}}

    {{ if eq $store.SyslogConfig.SSLVerify true}}
    verify_mode 1
    {{else -}}
    verify_mode 0
    {{end -}}

    {{ if $store.SyslogConfig.Certificate }}
    tls true        
    ca_file /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_ca.pem
    {{end}}

    {{ if and $store.SyslogConfig.ClientCert $store.SyslogConfig.ClientKey}}        
    client_cert /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-cert.pem
    client_cert_key /fluentd/etc/ssl/project_{{$store.WrapProjectName}}_client-key.pem
    {{ end -}}
    {{end}}
    
    <buffer>
      @type file
      path /fluentd/etc/buffer/project.{{$store.WrapProjectName}}.buffer
      flush_interval {{$store.OutputFlushInterval}}s
      {{ if eq $store.CurrentTarget "splunk"}}
      buffer_chunk_limit 8m
      {{end -}}
    </buffer> 

    disable_retry_limit
    num_threads 8
    slow_flush_log_threshold 40.0
</match>
{{end -}}
{{end -}}
`
