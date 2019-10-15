package generator

var MatchTemplate = `
{{define "match"}}
<match  {{ .ContainerLogSourceTag}}.** {{ .CustomLogSourceTag}}.** {{ if .IncludeRke }}{{ .RkeLogTag }}.**{{end}}> 
  @type copy
  {{- template "store-target" . -}}
  {{- template "store-prometheus" . -}}
</match>
{{end}}

{{define "store-target"}}
  <store>
  {{- template "elasticsearch" . -}}
  {{- template "splunk" . -}}
  {{- template "kafka" . -}}
  {{- template "syslog" . -}}
  {{- template "fluentforwarder" . -}}
  {{- template "custom" . -}}
  {{- template "buffer" . -}}
  </store>
{{end}}

{{define "store-prometheus"}}
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
{{end}}

{{define "elasticsearch"}}
{{ if eq .CurrentTarget "elasticsearch" }}
	@type elasticsearch
	include_tag_key  true
	{{- if and .ElasticsearchConfig.AuthUserName .ElasticsearchConfig.AuthPassword}}
	user {{.ElasticsearchConfig.AuthUserName}}
	password {{.ElasticsearchConfig.AuthPassword}}
	{{- end }}
	hosts {{.ElasticsearchConfig.Endpoint}}    
	logstash_prefix "{{.ElasticsearchConfig.IndexPrefix}}"
	logstash_format true
	logstash_dateformat  {{.ElasticsearchTemplateWrap.DateFormat}}
	type_name  "container_log"
	{{- if eq .ElasticsearchTemplateWrap.Scheme "https"}}
	ssl_verify {{.ElasticsearchConfig.SSLVerify}}
	ssl_version {{ .ElasticsearchConfig.SSLVersion }}
	{{- if .ElasticsearchConfig.Certificate }}
	ca_file {{.CertFilePrefix}}_ca.pem
	{{end}}
	{{- if and .ElasticsearchConfig.ClientCert .ElasticsearchConfig.ClientKey}}
	client_cert {{.CertFilePrefix}}_client-cert.pem
	client_key {{.CertFilePrefix}}_client-key.pem
	{{end}}
	{{- if .ElasticsearchConfig.ClientKeyPass}}
	client_key_pass {{.ElasticsearchConfig.ClientKeyPass}}
	{{end}}
	{{end}}
{{end}}
{{end}}

{{define "splunk"}}
{{- if eq .CurrentTarget "splunk"}}
	@type splunk_hec
	host {{.SplunkTemplateWrap.Host}}
	port {{.SplunkTemplateWrap.Port}}
	token {{.SplunkConfig.Token}}
	{{- if .SplunkConfig.Source}}
	sourcetype {{.SplunkConfig.Source}}
	{{end}}
	{{- if .SplunkConfig.Index}}
	default_index {{ .SplunkConfig.Index }}
	{{end}}
	{{- if eq .SplunkTemplateWrap.Scheme "https"}}
	use_ssl true    
	ssl_verify {{.SplunkConfig.SSLVerify}}    
	{{- if .SplunkConfig.Certificate }}    
	ca_file {{.CertFilePrefix}}_ca.pem
	{{end}}
	{{- if and .SplunkConfig.ClientCert .SplunkConfig.ClientKey}}    
	client_cert {{.CertFilePrefix}}_client-cert.pem
	client_key {{.CertFilePrefix}}_client-key.pem
	{{end}}
	{{- if .SplunkConfig.ClientKeyPass}}    
	client_key_pass {{ .SplunkConfig.ClientKeyPass }}
	{{end}}
	{{end}}
{{end}}
{{end}}

{{define "kafka"}}
{{- if eq .CurrentTarget "kafka"}}
	@type kafka_buffered
	{{- if .KafkaConfig.ZookeeperEndpoint }}
	zookeeper {{.KafkaTemplateWrap.Zookeeper}}
	{{else}}
	brokers {{.KafkaTemplateWrap.Brokers}}
	{{end}}
	default_topic {{.KafkaConfig.Topic}}
	output_data_type  "json"
	output_include_tag  true
	output_include_time  true
	max_send_retries 5
	kafka_agg_max_bytes 768000
	kafka_agg_max_messages 500
	get_kafka_client_log true 

	{{- if .KafkaConfig.Certificate }}        
	ssl_ca_cert {{.CertFilePrefix}}_ca.pem
	{{end}}
	{{- if and .KafkaConfig.ClientCert .KafkaConfig.ClientKey}}        
	ssl_client_cert {{.CertFilePrefix}}_client-cert.pem
	ssl_client_cert_key {{.CertFilePrefix}}_client-key.pem
	{{end}}
	{{- if and .KafkaConfig.SaslUsername .KafkaConfig.SaslPassword}}        
	username {{.KafkaConfig.SaslUsername}}
	password {{.KafkaConfig.SaslPassword}}
	{{end}}
	{{- if and (eq .KafkaConfig.SaslType "scram") .KafkaConfig.SaslScramMechanism}}        
	scram_mechanism {{.KafkaConfig.SaslScramMechanism}}
	{{- if eq .KafkaTemplateWrap.IsSSL false}}
	sasl_over_ssl false
	{{end}}
	{{end}}
{{end}}
{{end}}

{{define "syslog"}}
{{- if eq .CurrentTarget "syslog"}}
	@type remote_syslog
	host {{.SyslogTemplateWrap.Host}}
	port {{.SyslogTemplateWrap.Port}}
	severity {{.SyslogTemplateWrap.WrapSeverity}}
	protocol {{.SyslogConfig.Protocol}}
	packet_size 65535
	{{- if .SyslogConfig.Program }}
	program {{.SyslogConfig.Program}}
	{{end}}
	{{- if eq .SyslogConfig.SSLVerify true}}
	verify_mode 1
	{{else }}
	verify_mode 0
	{{end}}
	{{- if .SyslogConfig.EnableTLS }}
	tls true
	{{- if .SyslogConfig.Certificate }}
	ca_file {{.CertFilePrefix}}_ca.pem
	{{end}}
	{{end}}
	{{- if and .SyslogConfig.ClientCert .SyslogConfig.ClientKey}}        
	client_cert {{.CertFilePrefix}}_client-cert.pem
	client_cert_key {{.CertFilePrefix}}_client-key.pem
	{{end}}
{{end}}
{{end}}

{{define "fluentforwarder"}}
{{- if eq .CurrentTarget "fluentforwarder"}}
	@type forward
	{{- if .FluentForwarderConfig.EnableTLS }}
	transport tls    
	tls_allow_self_signed_cert true
	{{- if .FluentForwarderConfig.SSLVerify }}
	tls_verify_hostname true
	{{else }}
	tls_verify_hostname false
	{{end }}
	{{end}}
	{{- if .FluentForwarderConfig.Certificate }}
	tls_cert_path {{.CertFilePrefix}}_ca.pem
	{{end}}
	{{- if and .FluentForwarderConfig.ClientCert .FluentForwarderConfig.ClientKey}}
	tls_client_cert_path {{.CertFilePrefix}}_client-cert.pem
	tls_client_private_key_path {{.CertFilePrefix}}_client-key.pem
	{{end}}
	{{- if .FluentForwarderConfig.ClientKeyPass}}
	tls_client_private_key_passphrase {{.FluentForwarderConfig.ClientKeyPass}}
	{{end}}
	{{- if .FluentForwarderConfig.Compress }}
	compress gzip
	{{end}}
	{{- if .FluentForwarderTemplateWrap.EnableShareKey }}
	<security>
	  self_hostname "#{Socket.gethostname}"
	  shared_key true
	</security>
	{{end}}
	{{- range $k, $val := .FluentForwarderTemplateWrap.FluentServers }}
	<server>
	  {{if $val.Hostname}}
	  name {{$val.Hostname}}
	  {{end}}
	  host {{$val.Host}}
	  port {{$val.Port}}
	  {{ if $val.SharedKey}}
	  shared_key {{$val.SharedKey}}
	  {{end}}
	  {{ if $val.Username}}
	  username  {{$val.Username}}
	  {{end}}
	  {{ if $val.Password}}
	  password  {{$val.Password}}
	  {{end}}
	  weight  {{$val.Weight}}
	  {{if $val.Standby}}
	  standby
	  {{end}}
	</server>
	{{end}}
{{end}}
{{end}}

{{define "custom"}}
{{- if eq .CurrentTarget "customtarget"}}
{{.CustomTargetWrap.Content}} 
{{end}}
{{end}}

{{define "buffer"}}
	<buffer>
	  @type file
	  path /fluentd/log/buffer/{{.BufferFile}}
	  flush_mode interval
	  flush_interval {{.OutputFlushInterval}}s
	  flush_thread_count 16
	  {{- if eq .CurrentTarget "kafka"}}
	  chunk_limit_size 32m
	  {{end}}
	  {{- if eq .CurrentTarget "splunk"}}
	  chunk_limit_size 8m
	  {{end}}
	  queued_chunks_limit_size 300
	</buffer> 
	slow_flush_log_threshold 40.0	
{{end}}
`
