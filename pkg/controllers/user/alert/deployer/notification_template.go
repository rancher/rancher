package deployer

const (
	NotificationTmpl = `{{ define "rancher.title" }}
{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
{{ (index .Alerts 0).Labels.event_type}} event of {{(index .Alerts 0).Labels.resource_kind}} occurred

{{ else if eq (index .Alerts 0).Labels.alert_type "nodeHealthy"}}
The kubelet on the node {{ (index .Alerts 0).Labels.node_name}} is not healthy

{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
The system component {{ (index .Alerts 0).Labels.component_name}} is not running

{{ else if eq (index .Alerts 0).Labels.alert_type "nodeCPU"}}
The CPU usage on the node {{ (index .Alerts 0).Labels.node_name}} is over {{ (index .Alerts 0).Labels.cpu_threshold}}%

{{ else if eq (index .Alerts 0).Labels.alert_type "nodeMemory"}}
The memory usage on the node {{ (index .Alerts 0).Labels.node_name}} is over {{ (index .Alerts 0).Labels.mem_threshold}}%

{{ else if eq (index .Alerts 0).Labels.alert_type "podNotScheduled"}}
The Pod {{ if (index .Alerts 0).Labels.namespace}}{{(index .Alerts 0).Labels.namespace}}:{{end}}{{(index .Alerts 0).Labels.pod_name}} is not scheduled

{{ else if eq (index .Alerts 0).Labels.alert_type "podNotRunning"}}
The Pod {{ if (index .Alerts 0).Labels.namespace}}{{(index .Alerts 0).Labels.namespace}}:{{end}}{{(index .Alerts 0).Labels.pod_name}} is not running

{{ else if eq (index .Alerts 0).Labels.alert_type "podRestarts"}}
The Pod {{ if (index .Alerts 0).Labels.namespace}}{{(index .Alerts 0).Labels.namespace}}:{{end}}{{(index .Alerts 0).Labels.pod_name}} restarts {{ (index .Alerts 0).Labels.restart_times}} times in {{ (index .Alerts 0).Labels.restart_interval}} sec

{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
The system component {{ (index .Alerts 0).Labels.component_name}} is not running

{{ else if eq (index .Alerts 0).Labels.alert_type "workload"}}
The workload {{ if (index .Alerts 0).Labels.workload_namespace}}{{(index .Alerts 0).Labels.workload_namespace}}:{{end}}{{(index .Alerts 0).Labels.workload_name}} has available replicas less than {{ (index .Alerts 0).Labels.available_percentage}}%

{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
The metric {{ (index .Alerts 0).Labels.alert_name}} crossed the threshold 
{{ end}}
{{ end}}

{{ define "slack.text" }}
{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Target: {{ if (index .Alerts 0).Labels.target_namespace}}{{(index .Alerts 0).Labels.target_namespace}}:{{end}}{{(index .Alerts 0).Labels.target_name}}
Count: {{ (index .Alerts 0).Labels.event_count}}
Event Message: {{ (index .Alerts 0).Labels.event_message}}
First Seen: {{ (index .Alerts 0).Labels.event_firstseen}}
Last Seen: {{ (index .Alerts 0).Labels.event_lastseen}}
{{ else if eq (index .Alerts 0).Labels.alert_type "nodeHealthy"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "nodeCPU"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Used CPU: {{ (index .Alerts 0).Labels.used_cpu}} m
Total CPU: {{ (index .Alerts 0).Labels.total_cpu}} m
{{ else if eq (index .Alerts 0).Labels.alert_type "nodeMemory"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Used Memory: {{ (index .Alerts 0).Labels.used_mem}}
Total Memory: {{ (index .Alerts 0).Labels.total_mem}}
{{ else if eq (index .Alerts 0).Labels.alert_type "podRestarts"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Namespace: {{ (index .Alerts 0).Labels.namespace}}
Container Name: {{(index .Alerts 0).Labels.container_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "podNotRunning"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Namespace: {{ (index .Alerts 0).Labels.namespace}}
Container Name: {{ (index .Alerts 0).Labels.container_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "podNotScheduled"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Namespace: {{ (index .Alerts 0).Labels.namespace}}
Pod Name: {{ (index .Alerts 0).Labels.pod_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "workload"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Available Replicas: {{ (index .Alerts 0).Labels.available_replicas}}
Desired Replicas: {{ (index .Alerts 0).Labels.desired_replicas}}
{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Expression: {{(index .Alerts 0).Labels.expression}}
Description: Threshold Crossed: datapoint was {{ (index .Alerts 0).Labels.comparison}} to the threshold ({{ (index .Alerts 0).Labels.threshold_value}}) for ({{ (index .Alerts 0).Labels.duration}})
{{ end}}
{{ if (index .Alerts 0).Labels.logs}}
Logs: {{ (index .Alerts 0).Labels.logs}}
{{ end}}
{{ end}}

{{ define "email.text" }}
{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Target: {{ if (index .Alerts 0).Labels.target_namespace}}{{(index .Alerts 0).Labels.target_namespace}}:{{end}}{{ (index .Alerts 0).Labels.target_name}}<br>
Count: {{ (index .Alerts 0).Labels.event_count}}<br>
Event Message: {{ (index .Alerts 0).Labels.event_message}}<br>
First Seen: {{ (index .Alerts 0).Labels.event_firstseen}}<br>
Last Seen: {{ (index .Alerts 0).Labels.event_lastseen}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "nodeHealthy"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "nodeCPU"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Used CPU: {{ (index .Alerts 0).Labels.used_cpu}} m<br>
Total CPU: {{ (index .Alerts 0).Labels.total_cpu}} m<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "nodeMemory"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Used Memory: {{ (index .Alerts 0).Labels.used_mem}}<br>
Total Memory: {{ (index .Alerts 0).Labels.total_mem}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "podRestarts"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Namespace: {{ (index .Alerts 0).Labels.namespace}}<br>
Container Name: {{(index .Alerts 0).Labels.container_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "podNotRunning"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Namespace: {{ (index .Alerts 0).Labels.namespace}}<br>
Container Name: {{ (index .Alerts 0).Labels.container_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "podNotScheduled"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Namespace: {{ (index .Alerts 0).Labels.namespace}}<br>
Pod Name: {{ (index .Alerts 0).Labels.pod_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "workload"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Available Replicas: {{ (index .Alerts 0).Labels.available_replicas}}<br>
Desired Replicas: {{ (index .Alerts 0).Labels.desired_replicas}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Expression: {{(index .Alerts 0).Labels.expression}}<br>
Description: Threshold Crossed: datapoint was {{ (index .Alerts 0).Labels.comparison}} to the threshold ({{ (index .Alerts 0).Labels.threshold_value}}) for ({{ (index .Alerts 0).Labels.duration}})<br>
{{ end}}
{{ if (index .Alerts 0).Labels.logs}}
Logs: {{ (index .Alerts 0).Labels.logs}}
{{ end}}
{{ end}}
`
)
