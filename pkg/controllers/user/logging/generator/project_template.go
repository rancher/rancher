package generator

var ProjectTemplate = `{{range $i, $store := .projectTargets -}}
<source>
   @type  tail
   path  /var/log/containers/*.log
   pos_file  /fluentd/etc/log/fluentd-project-{{$store.ProjectName}}-logging.pos
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

{{ if $store.CurrentTarget }}
<match  {{$store.ProjectName}}.**> 
    flush_interval {{$store.OutputFlushInterval}}s
    {{ if eq $store.CurrentTarget "elasticsearch"}}
    @type elasticsearch
    include_tag_key  true
    hosts {{$store.ElasticsearchConfig.Endpoint}}
    reload_connections "true"
    logstash_prefix "{{$store.ElasticsearchConfig.IndexPrefix}}"
    logstash_format true
    logstash_dateformat  {{$store.WrapElasticsearch.DateFormat}}
    type_name  "container_log"
    {{end -}}

    {{ if eq $store.CurrentTarget "splunk"}}
    @type splunk-http-eventcollector
    server  {{$store.WrapSplunk.Server}}
    all_items true
    protocol {{$store.WrapSplunk.Scheme}}
    sourcetype {{$store.SplunkConfig.Source}}
    format json
    token {{$store.SplunkConfig.Token}}
    reload_connections "true"
    {{end -}}

    {{ if eq $store.CurrentTarget "kafka"}}
    @type kafka_buffered
    {{ if $store.KafkaConfig.ZookeeperEndpoint }}
    zookeeper {{$store.KafkaConfig.ZookeeperEndpoint}}
    {{else}}
    brokers {{$store.KafkaConfig.BrokerEndpoints}}
    {{end}}
    default_topic {{$store.KafkaConfig.Topic}}
    output_data_type  "json"
    output_include_tag  true
    output_include_time  true
    # get_kafka_client_log  true
    max_send_retries 3
    {{end -}}

    {{ if eq $store.CurrentTarget "syslog"}}
    @type remote_syslog
    host {{$store.WrapSyslog.Host}}
    port {{$store.WrapSyslog.Port}}
    severity {{$store.SyslogConfig.Severity}}
    program {{$store.SyslogConfig.Program}}
    {{end}}
    

    max_retry_wait 30
    disable_retry_limit
    num_threads 8
    buffer_type file
    buffer_path /fluentd/etc/buffer/project.{{$store.ProjectName}}.buffer
    buffer_queue_limit 128
    buffer_chunk_limit 256m
    slow_flush_log_threshold 40.0
</match>
{{end -}}
{{end -}}
`
