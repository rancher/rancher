package generator

var ClusterTemplate = `{{ if .clusterTarget.CurrentTarget }}

<source>
  @type  tail
  path  /var/lib/rancher/rke/log/*.log
  pos_file  /fluentd/log/fluentd-rke-logging.pos
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
    {{range $k, $val := .clusterTarget.OutputTags -}}
    {{$k}} {{$val}}
    {{end -}}
  </record>
</filter>

{{ if eq .clusterTarget.CurrentTarget "syslog"}}
{{ if .clusterTarget.SyslogConfig.Token}}
<filter  cluster.** rke.** cluster-custom.**>
  @type record_transformer
  <record>
    tag ${tag} {{.clusterTarget.SyslogConfig.Token}}
  </record>
</filter>
{{end -}}
{{end -}}

<match  cluster.** rke.** cluster-custom.**> 
    {{ if eq .clusterTarget.CurrentTarget "embedded"}}
    @type elasticsearch
    include_tag_key  true
    hosts "elasticsearch.cattle-logging:9200"
    logstash_prefix {{.clusterTarget.EmbeddedConfig.IndexPrefix}}
    logstash_format true
    logstash_dateformat  {{.clusterTarget.WrapEmbedded.DateFormat}}
    type_name  "container_log"
    {{end -}}

    {{ if eq .clusterTarget.CurrentTarget "elasticsearch"}}
    @type elasticsearch
    include_tag_key  true
    {{ if and .clusterTarget.ElasticsearchConfig.AuthUserName .clusterTarget.ElasticsearchConfig.AuthPassword}}
    hosts {{.clusterTarget.WrapElasticsearch.Scheme}}://{{.clusterTarget.ElasticsearchConfig.AuthUserName}}:{{.clusterTarget.ElasticsearchConfig.AuthPassword}}@{{.clusterTarget.WrapElasticsearch.Host}}
    {{else -}}
    hosts {{.clusterTarget.ElasticsearchConfig.Endpoint}}    
    {{end -}}
    logstash_format true
    logstash_prefix "{{.clusterTarget.ElasticsearchConfig.IndexPrefix}}"
    logstash_dateformat  {{.clusterTarget.WrapElasticsearch.DateFormat}}
    type_name  "container_log"

    {{ if eq .clusterTarget.WrapElasticsearch.Scheme "https"}}    
    ssl_verify {{ .clusterTarget.ElasticsearchConfig.SSLVerify }}
    
    {{ if .clusterTarget.ElasticsearchConfig.Certificate }}
    ca_file /fluentd/etc/ssl/cluster_{{.clusterName}}_ca.pem
    {{end -}}

    {{ if and .clusterTarget.ElasticsearchConfig.ClientCert .clusterTarget.ElasticsearchConfig.ClientKey}}
    client_cert /fluentd/etc/ssl/cluster_{{.clusterName}}_client-cert.pem
    client_key /fluentd/etc/ssl/cluster_{{.clusterName}}_client-key.pem
    {{end -}}

    {{ if .clusterTarget.ElasticsearchConfig.ClientKeyPass}}
    client_key_pass {{.clusterTarget.ElasticsearchConfig.ClientKeyPass}}
    {{end -}}
    {{end -}}
    {{end -}}

    {{ if eq .clusterTarget.CurrentTarget "splunk"}}
    @type splunk_hec
    host {{.clusterTarget.WrapSplunk.Host}}
    port {{.clusterTarget.WrapSplunk.Port}}
    token {{.clusterTarget.SplunkConfig.Token}}

    {{ if .clusterTarget.SplunkConfig.Source}}
    sourcetype {{.clusterTarget.SplunkConfig.Source}}
    {{end -}}
    {{ if .clusterTarget.SplunkConfig.Index}}
    default_index {{ .clusterTarget.SplunkConfig.Index }}
    {{end -}}

    {{ if eq .clusterTarget.WrapSplunk.Scheme "https"}}
    use_ssl true
    ssl_verify {{ .clusterTarget.SplunkConfig.SSLVerify }}

    {{ if .clusterTarget.SplunkConfig.Certificate }}    
    ca_file /fluentd/etc/ssl/cluster_{{.clusterName}}_ca.pem
    {{end -}}

    {{ if and .clusterTarget.SplunkConfig.ClientCert .clusterTarget.SplunkConfig.ClientKey}}    
    client_cert /fluentd/etc/ssl/cluster_{{.clusterName}}_client-cert.pem
    client_key /fluentd/etc/ssl/cluster_{{.clusterName}}_client-key.pem
    {{end -}}

    {{ if .clusterTarget.SplunkConfig.ClientKeyPass}}    
    client_key_pass {{ .clusterTarget.SplunkConfig.ClientKeyPass }}
    {{end -}}
    {{end -}}
    {{end -}}

    {{ if eq .clusterTarget.CurrentTarget "kafka"}}
    @type kafka_buffered
    {{ if .clusterTarget.KafkaConfig.ZookeeperEndpoint }}
    zookeeper {{.clusterTarget.WrapKafka.Zookeeper}}
    {{else}}
    brokers {{.clusterTarget.WrapKafka.Brokers}}
    {{end}}
    default_topic {{.clusterTarget.KafkaConfig.Topic}}
    output_data_type  "json"
    output_include_tag true
    output_include_time true

    {{ if .clusterTarget.KafkaConfig.Certificate }}        
    ssl_ca_cert /fluentd/etc/ssl/cluster_{{.clusterName}}_ca.pem
    {{end}}

    {{ if and .clusterTarget.KafkaConfig.ClientCert .clusterTarget.KafkaConfig.ClientKey}}        
    ssl_client_cert /fluentd/etc/ssl/cluster_{{.clusterName}}_client-cert.pem
    ssl_client_cert_key /fluentd/etc/ssl/cluster_{{.clusterName}}_client-key.pem
    {{ end -}}
    max_send_retries  3
    {{end -}}

    {{ if eq .clusterTarget.CurrentTarget "syslog"}}
    @type remote_syslog
    host {{.clusterTarget.WrapSyslog.Host}}
    port {{.clusterTarget.WrapSyslog.Port}}
    severity {{.clusterTarget.SyslogConfig.Severity}}
    protocol {{.clusterTarget.SyslogConfig.Protocol}}
    {{ if .clusterTarget.SyslogConfig.Program }}
    program {{.clusterTarget.SyslogConfig.Program}}
    {{end -}}
    packet_size 65535
    
    {{ if eq .clusterTarget.SyslogConfig.SSLVerify true}}
    verify_mode 1
    {{else -}}
    verify_mode 0
    {{end -}}

    {{ if .clusterTarget.SyslogConfig.Certificate }}
    tls true        
    ca_file /fluentd/etc/ssl/cluster_{{.clusterName}}_ca.pem
    {{end}}

    {{ if and .clusterTarget.SyslogConfig.ClientCert .clusterTarget.SyslogConfig.ClientKey}}        
    client_cert /fluentd/etc/ssl/cluster_{{.clusterName}}_client-cert.pem
    client_cert_key /fluentd/etc/ssl/cluster_{{.clusterName}}_client-key.pem
    {{ end -}}
    {{end -}}

    {{ if eq .clusterTarget.CurrentTarget "fluentforwarder"}}
    @type forward
    {{ if .clusterTarget.FluentForwarderConfig.EnableTLS }}
    transport tls  
    tls_verify_hostname true
    tls_allow_self_signed_cert true
    {{end -}}    
    {{ if .clusterTarget.FluentForwarderConfig.Certificate }}
    tls_cert_path /fluentd/etc/ssl/cluster_{{.clusterName}}_ca.pem
    {{end -}}  
    
    {{ if .clusterTarget.FluentForwarderConfig.Compress }}
    compress gzip
    {{end -}}

    {{ if .clusterTarget.WrapFluentForwarder.EnableShareKey }}
    <security>
      self_hostname "#{Socket.gethostname}"
      shared_key true
    </security>
    {{end -}}

    {{range $k, $val := .clusterTarget.WrapFluentForwarder.FluentServers -}}
    <server>
      {{if $val.Hostname}}
      name {{$val.Hostname}}
      {{end -}}
      host {{$val.Host}}
      port {{$val.Port}}
      {{ if $val.SharedKey}}
      shared_key {{$val.SharedKey}}
      {{end -}}
      {{ if $val.Username}}
      username  {{$val.Username}}
      {{end -}}
      {{ if $val.Password}}
      password  {{$val.Password}}
      {{end -}}
      weight  {{$val.Weight}}
      {{if $val.Standby}}
      standby
      {{end -}}
      
    </server>
    {{end -}}
    {{end -}}    

    <buffer>
      @type file
      path /fluentd/etc/buffer/cluster.buffer
      flush_interval {{.clusterTarget.OutputFlushInterval}}s
      {{ if eq .clusterTarget.CurrentTarget "splunk"}}
      chunk_limit_size 8m
      {{end -}}
    </buffer> 

    disable_retry_limit
    num_threads 8
    slow_flush_log_threshold 40.0
</match>
{{end -}}
`
