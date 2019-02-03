package generator

var ProjectTemplate = `
{{ range $i, $store := .projectTargets }}
{{- if $store.CurrentTarget }}
 
  {{- if eq $store.IsSystemProject true }}
  <source>
    @type  tail
    path  /var/lib/rancher/rke/log/*.log
    pos_file  /fluentd/log/fluentd-rke-logging-system-project.pos
    time_format  %Y-%m-%dT%H:%M:%S
    tag  rke.*
    format  json
    read_from_head  true
  </source>

  <filter rke.**>
    @type record_transformer
    enable_ruby true  
    <record>
      tag ${tag}
      log_type k8s_infrastructure_container 
      driver rke
      component ${tag_suffix[6].split("_")[0]}
      container_id ${tag_suffix[6].split(".")[0]}
    </record>
  </filter>
  {{end }}

  <source>
     @type  tail
     path  /var/log/containers/*.log
     pos_file  /fluentd/log/fluentd-project-{{$store.ProjectName}}-logging.pos
     time_format  %Y-%m-%dT%H:%M:%S
     tag  {{$store.ProjectName}}.*
     format  json
     read_from_head  true
  </source>

  <filter {{$store.ProjectName}}.**>
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

  {{- if eq $store.CurrentTarget "syslog"}}
  {{- if $store.SyslogConfig.Token}}
  <filter {{$store.ProjectName}}.** project-custom.{{$store.ProjectName}}.** {{ if eq $store.IsSystemProject true }}rke.**{{end }} >
    @type record_transformer
    <record>
      tag ${tag} {{$store.SyslogConfig.Token}}
    </record>
  </filter>
  {{end }}
  {{end }}

  <filter {{$store.ProjectName}}.**>
    @type prometheus
    <metric>
      name fluentd_input_status_num_records_total
      type counter
      desc The total number of incoming records
      <labels>
        tag ${tag}
        hostname ${hostname}
      </labels>
    </metric>
  </filter>

  <match  {{$store.ProjectName}}.** project-custom.{{$store.ProjectName}}.** {{ if eq $store.IsSystemProject true }}rke.**{{end }}> 
    @type copy
    <store>
      {{- if or (eq $store.CurrentTarget "elasticsearch") (eq $store.CurrentTarget "splunk") (eq $store.CurrentTarget "syslog") (eq $store.CurrentTarget "kafka") (eq $store.CurrentTarget "fluentforwarder")}}
  
      {{- if eq $store.CurrentTarget "elasticsearch"}}
      @type elasticsearch
      include_tag_key  true
      {{- if and $store.ElasticsearchConfig.AuthUserName $store.ElasticsearchConfig.AuthPassword}}
      hosts {{$store.ElasticsearchTemplateWrap.Scheme}}://{{$store.ElasticsearchConfig.AuthUserName}}:{{$store.ElasticsearchConfig.AuthPassword}}@{{$store.ElasticsearchTemplateWrap.Host}}
      {{else }}
      hosts {{$store.ElasticsearchConfig.Endpoint}}    
      {{end }}

      logstash_prefix "{{$store.ElasticsearchConfig.IndexPrefix}}"
      logstash_format true
      logstash_dateformat  {{$store.ElasticsearchTemplateWrap.DateFormat}}
      type_name  "container_log"

      {{- if eq $store.ElasticsearchTemplateWrap.Scheme "https"}}
      ssl_verify {{$store.ElasticsearchConfig.SSLVerify}}
      ssl_version {{ $store.ElasticsearchConfig.SSLVersion }}

      {{- if $store.ElasticsearchConfig.Certificate }}
      ca_file {{ $.certDir}}/project_{{$store.WrapProjectName}}_ca.pem
      {{end }}

      {{- if and $store.ElasticsearchConfig.ClientCert $store.ElasticsearchConfig.ClientKey}}
      client_cert {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-cert.pem
      client_key {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-key.pem
      {{end }}

      {{- if $store.ElasticsearchConfig.ClientKeyPass}}
      client_key_pass {{$store.ElasticsearchConfig.ClientKeyPass}}
      {{end }}
      {{end }}
      {{end }}

      {{- if eq $store.CurrentTarget "splunk"}}
      @type splunk_hec
      host {{$store.SplunkTemplateWrap.Host}}
      port {{$store.SplunkTemplateWrap.Port}}
      token {{$store.SplunkConfig.Token}}
      {{- if $store.SplunkConfig.Source}}
      sourcetype {{$store.SplunkConfig.Source}}
      {{end }}
      {{- if $store.SplunkConfig.Index}}
      default_index {{ $store.SplunkConfig.Index }}
      {{end }}

      {{- if eq $store.SplunkTemplateWrap.Scheme "https"}}
      use_ssl true    
      ssl_verify {{$store.SplunkConfig.SSLVerify}}    

      {{- if $store.SplunkConfig.Certificate }}    
      ca_file {{ $.certDir}}/project_{{$store.WrapProjectName}}_ca.pem
      {{end }}

      {{- if and $store.SplunkConfig.ClientCert $store.SplunkConfig.ClientKey}}    
      client_cert {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-cert.pem
      client_key {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-key.pem
      {{end }}

      {{- if $store.SplunkConfig.ClientKeyPass}}    
      client_key_pass {{ $store.SplunkConfig.ClientKeyPass }}
      {{end }}
      {{end }}
      {{end }}

      {{- if eq $store.CurrentTarget "kafka"}}
      @type kafka_buffered
      {{- if $store.KafkaConfig.ZookeeperEndpoint }}
      zookeeper {{$store.KafkaTemplateWrap.Zookeeper}}
      {{else}}
      brokers {{$store.KafkaTemplateWrap.Brokers}}
      {{end }}
      default_topic {{$store.KafkaConfig.Topic}}
      output_data_type  "json"
      output_include_tag  true
      output_include_time  true
      max_send_retries 3

      {{- if $store.KafkaConfig.Certificate }}        
      ssl_ca_cert {{ $.certDir}}/project_{{$store.WrapProjectName}}_ca.pem
      {{end }}

      {{- if and $store.KafkaConfig.ClientCert $store.KafkaConfig.ClientKey}}        
      ssl_client_cert {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-cert.pem
      ssl_client_cert_key {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-key.pem
      {{end }}

      {{- if and $store.KafkaConfig.SaslUsername $store.KafkaConfig.SaslPassword}}        
      username {{$store.KafkaConfig.SaslUsername}}
      password {{$store.KafkaConfig.SaslPassword}}
      {{end }}
  
      {{- if and (eq $store.KafkaConfig.SaslType "scram") $store.KafkaConfig.SaslScramMechanism}}        
      scram_mechanism {{$store.KafkaConfig.SaslScramMechanism}}
      {{- if eq $store.KafkaTemplateWrap.IsSSL false}}
      sasl_over_ssl false
      {{end}}
      {{end }}
      {{end }}

      {{- if eq $store.CurrentTarget "syslog"}}
      @type remote_syslog
      host {{$store.SyslogTemplateWrap.Host}}
      port {{$store.SyslogTemplateWrap.Port}}
      severity {{$store.SyslogConfig.Severity}}
      protocol {{$store.SyslogConfig.Protocol}}
      {{- if $store.SyslogConfig.Program }}
      program {{$store.SyslogConfig.Program}}
      {{end }}
      packet_size 65535

      {{- if eq $store.SyslogConfig.SSLVerify true}}
      verify_mode 1
      {{else }}
      verify_mode 0
      {{end }}

      {{- if $store.SyslogConfig.EnableTLS }}
      tls true
      {{- if $store.SyslogConfig.Certificate }}
      ca_file {{ $.certDir}}/project_{{$store.WrapProjectName}}_ca.pem
      {{end }}
      {{end }}

      {{- if and $store.SyslogConfig.ClientCert $store.SyslogConfig.ClientKey}}        
      client_cert {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-cert.pem
      client_cert_key {{ $.certDir}}/project_{{$store.WrapProjectName}}_client-key.pem
      {{end }}
      {{end }}

      {{- if eq $store.CurrentTarget "fluentforwarder"}}
      @type forward
      {{- if $store.FluentForwarderConfig.EnableTLS }}
      transport tls    
      tls_allow_self_signed_cert true
      tls_verify_hostname true
      {{end }}
      {{- if $store.FluentForwarderConfig.Certificate }}
      tls_cert_path {{ $.certDir}}/project_{{$store.WrapProjectName}}_ca.pem
      {{end }}  

      {{- if $store.FluentForwarderConfig.Compress }}
      compress gzip
      {{end }}

      {{- if $store.FluentForwarderTemplateWrap.EnableShareKey }}
      <security>
        self_hostname "#{Socket.gethostname}"
        shared_key true
      </security>
      {{end }}

      {{- range $k, $val := $store.FluentForwarderTemplateWrap.FluentServers }}
      <server>
        {{if $val.Hostname}}
        name {{$val.Hostname}}
        {{end }}
        host {{$val.Host}}
        port {{$val.Port}}
        {{ if $val.SharedKey}}
        shared_key {{$val.SharedKey}}
        {{end }}
        {{ if $val.Username}}
        username  {{$val.Username}}
        {{end }}
        {{ if $val.Password}}
        password  {{$val.Password}}
        {{end }}
        weight  {{$val.Weight}}
        {{if $val.Standby}}
        standby
        {{end }}
      </server>
      {{end }}
      {{end }}   

      <buffer>
        @type file
        path /fluentd/log/buffer/project.{{$store.WrapProjectName}}.buffer
        flush_mode interval
        flush_interval {{$store.OutputFlushInterval}}s
        flush_thread_count 8
        {{- if eq $store.CurrentTarget "splunk"}}
        chunk_limit_size 8m
        {{end }}
        queued_chunks_limit_size 200
      </buffer> 

      slow_flush_log_threshold 40.0
      {{end }}

      {{- if eq $store.CurrentTarget "customtarget"}}
      {{$store.CustomTargetWrap.Content}} 
      {{end }}
    </store>

    <store>
    @type prometheus
    <metric>
      name fluentd_output_status_num_records_total
      type counter
      desc The total number of outgoing records
      <labels>
        tag ${tag}
        hostname ${hostname}
      </labels>
    </metric>
    </store>
  </match>

{{end }}
{{end }}
`
