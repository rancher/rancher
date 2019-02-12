package generator

var ClusterTemplate = `{{ if .clusterTarget.CurrentTarget }}

{{- if eq .clusterTarget.IncludeSystemComponent true }}
<source>
  @type  tail
  path  /var/lib/rancher/rke/log/*.log
  pos_file  /fluentd/log/fluentd-rke-logging.pos
  time_format  %Y-%m-%dT%H:%M:%S.%N
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

<filter rke.*>
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
{{end }}

<source>
   @type  tail
   path  /var/log/containers/*.log
   pos_file  /fluentd/log/fluentd-cluster-logging.pos
   time_format  %Y-%m-%dT%H:%M:%S
   tag  cluster.*
   format  json
   read_from_head  true
</source>

<filter  cluster.**>
   @type  kubernetes_metadata
   merge_json_log  true
   preserve_json_log  true
</filter>

<filter cluster.**>
  @type record_transformer
  <record>
    tag ${tag}
    log_type k8s_normal_container 
    {{- range $k, $val := .clusterTarget.OutputTags }}
    {{$k}} {{$val}}
    {{end }}
  </record>
</filter>

<filter cluster.**>
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

{{- if eq .clusterTarget.IncludeSystemComponent false }}
<filter cluster.**>
  @type grep
  <exclude>
    key $.kubernetes.namespace_name
    pattern {{.clusterTarget.ExcludeNamespace}}
  </exclude>
</filter>
{{end }}

{{- if eq .clusterTarget.CurrentTarget "syslog"}}
{{- if .clusterTarget.SyslogConfig.Token}}
<filter  cluster.** cluster-custom.** {{ if eq .clusterTarget.IncludeSystemComponent true }}rke.**{{end }} >
  @type record_transformer
  <record>
    tag ${tag} {{.clusterTarget.SyslogConfig.Token}}
  </record>
</filter>
{{end }}
{{end }}

<match  cluster.** cluster-custom.** {{ if eq .clusterTarget.IncludeSystemComponent true }}rke.**{{end }} > 
  @type copy
  <store>
    {{- if or (eq .clusterTarget.CurrentTarget "elasticsearch") (eq .clusterTarget.CurrentTarget "splunk") (eq .clusterTarget.CurrentTarget "syslog") (eq .clusterTarget.CurrentTarget "kafka") (eq .clusterTarget.CurrentTarget "fluentforwarder")}}

    {{- if eq .clusterTarget.CurrentTarget "elasticsearch"}}
    @type elasticsearch
    include_tag_key  true
    {{- if and .clusterTarget.ElasticsearchConfig.AuthUserName .clusterTarget.ElasticsearchConfig.AuthPassword}}
    hosts {{.clusterTarget.ElasticsearchTemplateWrap.Scheme}}://{{.clusterTarget.ElasticsearchConfig.AuthUserName}}:{{.clusterTarget.ElasticsearchConfig.AuthPassword}}@{{.clusterTarget.ElasticsearchTemplateWrap.Host}}
    {{- else }}
    hosts {{.clusterTarget.ElasticsearchConfig.Endpoint}}    
    {{end }}
    logstash_format true
    logstash_prefix "{{.clusterTarget.ElasticsearchConfig.IndexPrefix}}"
    logstash_dateformat  {{.clusterTarget.ElasticsearchTemplateWrap.DateFormat}}
    type_name  "container_log"

    {{- if eq .clusterTarget.ElasticsearchTemplateWrap.Scheme "https"}}    
    ssl_verify {{ .clusterTarget.ElasticsearchConfig.SSLVerify }}
    ssl_version {{ .clusterTarget.ElasticsearchConfig.SSLVersion }}
    {{- if .clusterTarget.ElasticsearchConfig.Certificate }}
    ca_file {{ .certDir}}/cluster_{{.clusterName}}_ca.pem
    {{end }}

    {{- if and .clusterTarget.ElasticsearchConfig.ClientCert .clusterTarget.ElasticsearchConfig.ClientKey}}
    client_cert {{ .certDir}}/cluster_{{.clusterName}}_client-cert.pem
    client_key {{ .certDir}}/cluster_{{.clusterName}}_client-key.pem
    {{end }}

    {{- if .clusterTarget.ElasticsearchConfig.ClientKeyPass}}
    client_key_pass {{.clusterTarget.ElasticsearchConfig.ClientKeyPass}}
    {{end }}
    {{end }}
    {{end }}

    {{- if eq .clusterTarget.CurrentTarget "splunk"}}
    @type splunk_hec
    host {{.clusterTarget.SplunkTemplateWrap.Host}}
    port {{.clusterTarget.SplunkTemplateWrap.Port}}
    token {{.clusterTarget.SplunkConfig.Token}}

    {{- if .clusterTarget.SplunkConfig.Source}}
    sourcetype {{.clusterTarget.SplunkConfig.Source}}
    {{end }}
    {{- if .clusterTarget.SplunkConfig.Index}}
    default_index {{ .clusterTarget.SplunkConfig.Index }}
    {{end }}

    {{- if eq .clusterTarget.SplunkTemplateWrap.Scheme "https"}}
    use_ssl true
    ssl_verify {{ .clusterTarget.SplunkConfig.SSLVerify }}

    {{- if .clusterTarget.SplunkConfig.Certificate }}    
    ca_file {{ .certDir}}/cluster_{{.clusterName}}_ca.pem
    {{end }}

    {{- if and .clusterTarget.SplunkConfig.ClientCert .clusterTarget.SplunkConfig.ClientKey}}    
    client_cert {{ .certDir}}/cluster_{{.clusterName}}_client-cert.pem
    client_key {{ .certDir}}/cluster_{{.clusterName}}_client-key.pem
    {{end }}

    {{- if .clusterTarget.SplunkConfig.ClientKeyPass}}    
    client_key_pass {{ .clusterTarget.SplunkConfig.ClientKeyPass }}
    {{end }}
    {{end }}
    {{end }}

    {{- if eq .clusterTarget.CurrentTarget "kafka"}}
    @type kafka_buffered
    {{- if .clusterTarget.KafkaConfig.ZookeeperEndpoint }}
    zookeeper {{.clusterTarget.KafkaTemplateWrap.Zookeeper}}
    {{else}}
    brokers {{.clusterTarget.KafkaTemplateWrap.Brokers}}
    {{end }}
    default_topic {{.clusterTarget.KafkaConfig.Topic}}
    output_data_type  "json"
    output_include_tag true
    output_include_time true
    max_send_retries  3

    {{- if .clusterTarget.KafkaConfig.Certificate }}        
    ssl_ca_cert {{ .certDir}}/cluster_{{.clusterName}}_ca.pem
    {{end }}

    {{- if and .clusterTarget.KafkaConfig.ClientCert .clusterTarget.KafkaConfig.ClientKey}}        
    ssl_client_cert {{ .certDir}}/cluster_{{.clusterName}}_client-cert.pem
    ssl_client_cert_key {{ .certDir}}/cluster_{{.clusterName}}_client-key.pem
    {{end }}
    
    {{- if and .clusterTarget.KafkaConfig.SaslUsername .clusterTarget.KafkaConfig.SaslPassword}}        
    username {{.clusterTarget.KafkaConfig.SaslUsername}}
    password {{.clusterTarget.KafkaConfig.SaslPassword}}
    {{end }}

    {{- if and (eq .clusterTarget.KafkaConfig.SaslType "scram") .clusterTarget.KafkaConfig.SaslScramMechanism}}        
    scram_mechanism {{.clusterTarget.KafkaConfig.SaslScramMechanism}}
    {{- if eq .clusterTarget.KafkaTemplateWrap.IsSSL false}}
    sasl_over_ssl false
    {{end}}
    {{end }}

    {{end }}

    {{- if eq .clusterTarget.CurrentTarget "syslog"}}
    @type remote_syslog
    host {{.clusterTarget.SyslogTemplateWrap.Host}}
    port {{.clusterTarget.SyslogTemplateWrap.Port}}
    severity {{.clusterTarget.SyslogConfig.Severity}}
    protocol {{.clusterTarget.SyslogConfig.Protocol}}
    {{- if .clusterTarget.SyslogConfig.Program }}
    program {{.clusterTarget.SyslogConfig.Program}}
    {{end }}
    packet_size 65535
    
    {{- if eq .clusterTarget.SyslogConfig.SSLVerify true}}
    verify_mode 1
    {{else }}
    verify_mode 0
    {{end }}

    {{- if .clusterTarget.SyslogConfig.EnableTLS }}
    tls true    
    {{- if .clusterTarget.SyslogConfig.Certificate }}    
    ca_file {{ .certDir}}/cluster_{{.clusterName}}_ca.pem
    {{end }}
    {{end }}

    {{- if and .clusterTarget.SyslogConfig.ClientCert .clusterTarget.SyslogConfig.ClientKey}}        
    client_cert {{ .certDir}}/cluster_{{.clusterName}}_client-cert.pem
    client_cert_key {{ .certDir}}/cluster_{{.clusterName}}_client-key.pem
    {{end }}
    {{end }}

    {{- if eq .clusterTarget.CurrentTarget "fluentforwarder"}}
    @type forward
    {{- if .clusterTarget.FluentForwarderConfig.EnableTLS }}
    transport tls  
    tls_verify_hostname true
    tls_allow_self_signed_cert true
    {{end }}    
    {{ if .clusterTarget.FluentForwarderConfig.Certificate }}
    tls_cert_path {{ .certDir}}/cluster_{{.clusterName}}_ca.pem
    {{end }}  
    
    {{- if .clusterTarget.FluentForwarderConfig.Compress }}
    compress gzip
    {{end }}

    {{- if .clusterTarget.FluentForwarderTemplateWrap.EnableShareKey }}
    <security>
      self_hostname "#{Socket.gethostname}"
      shared_key true
    </security>
    {{end }}

    {{range $k, $val := .clusterTarget.FluentForwarderTemplateWrap.FluentServers }}
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
      path /fluentd/log/buffer/cluster.buffer
      flush_interval {{.clusterTarget.OutputFlushInterval}}s
      flush_mode interval
      flush_thread_count 8
      {{- if eq .clusterTarget.CurrentTarget "splunk"}}
      chunk_limit_size 8m
      queued_chunks_limit_size 200
      
      {{end }}
    </buffer> 

    slow_flush_log_threshold 40.0
    {{end }}
    {{- if eq .clusterTarget.CurrentTarget "customtarget"}}
    {{.clusterTarget.CustomTargetWrap.Content}} 
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
`
