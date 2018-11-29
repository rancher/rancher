package deployer

const (
	NotificationTmpl = `{{ define "rancher.title" }}
{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
{{ (index .Alerts 0).Labels.event_type}} event of {{(index .Alerts 0).Labels.resource_kind}} occurred

{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
The system component {{ (index .Alerts 0).Labels.component_name}} is not running

{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
The metric {{ (index .Alerts 0).Labels.metric}} for 

{{ if eq (index .Alerts 0).Labels.resource_type "cluster" }}
cluster {{ (index .Alerts 0).Labels.cluster_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "node" }}
node {{ (index .Alerts 0).Labels.instance }}
{{ else if eq (index .Alerts 0).Labels.resource_type "workload" }}
workload {{ (index .Alerts 0).Labels.pod_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "pod" }}
pod {{ (index .Alerts 0).Labels.pod_name }} is {{ (index .Alerts 0).Labels.conditionThreshold_comparison }} the threshold of {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue }}.
{{ end}}


is {{ (index .Alerts 0).Labels.conditionThreshold_comparison }} the threshold of {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue }}.
{{ end}}
{{ end}}

{{ define "slack.text" }}
{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Target: {{ (index .Alerts 0).Labels.target_name}}
Count: {{ (index .Alerts 0).Labels.event_count}}
Event Message: {{ (index .Alerts 0).Labels.event_message}}
First Seen: {{ (index .Alerts 0).Labels.event_firstseen}}
Last Seen: {{ (index .Alerts 0).Labels.event_lastseen}}
{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Resource Type: {{(index .Alerts 0).Labels.resource_type}}
Resource: {{ if eq (index .Alerts 0).Labels.resource_type "cluster" }}
cluster {{ (index .Alerts 0).Labels.cluster_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "node" }}
node {{ (index .Alerts 0).Labels.instance }}
{{ else if eq (index .Alerts 0).Labels.resource_type "workload" }}
workload {{ (index .Alerts 0).Labels.pod_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "pod" }}
pod {{ (index .Alerts 0).Labels.pod_name }}
{{ end}}
Metric: {{ (index .Alerts 0).Labels.metric}}
Comparison: {{ (index .Alerts 0).Labels.conditionThreshold_comparison}}
Threshold: {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue}}
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
Target: {{ (index .Alerts 0).Labels.target_name}}<br>
Count: {{ (index .Alerts 0).Labels.event_count}}<br>
Event Message: {{ (index .Alerts 0).Labels.event_message}}<br>
First Seen: {{ (index .Alerts 0).Labels.event_firstseen}}<br>
Last Seen: {{ (index .Alerts 0).Labels.event_lastseen}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Resource Type: {{(index .Alerts 0).Labels.resource_type}}<br>
Resource: {{ if eq (index .Alerts 0).Labels.resource_type "cluster" }}
cluster {{ (index .Alerts 0).Labels.cluster_name }}<br>
{{ else if eq (index .Alerts 0).Labels.resource_type "node" }}
node {{ (index .Alerts 0).Labels.instance }}<br>
{{ else if eq (index .Alerts 0).Labels.resource_type "workload" }}
workload {{ (index .Alerts 0).Labels.pod_name }}<br>
{{ else if eq (index .Alerts 0).Labels.resource_type "pod" }}
pod {{ (index .Alerts 0).Labels.pod_name }}<br>
{{ end}}
Metric: {{ (index .Alerts 0).Labels.metric}}<br>
Comparison: {{ (index .Alerts 0).Labels.conditionThreshold_comparison}}<br>
Threshold: {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue}}<br>
{{ end}}
{{ if (index .Alerts 0).Labels.logs}}
Logs: {{ (index .Alerts 0).Labels.logs}}
{{ end}}
{{ end}}
`
	Title = `
{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
{{ (index .Alerts 0).Labels.event_type}} event of {{(index .Alerts 0).Labels.resource_kind}} occurred

{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
The system component {{ (index .Alerts 0).Labels.component_name}} is not running

{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
The metric {{ (index .Alerts 0).Labels.metric}} for {{ if eq (index .Alerts 0).Labels.resource_type "cluster" }}
cluster {{ (index .Alerts 0).Labels.cluster_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "node" }}
node {{ (index .Alerts 0).Labels.instance }}
{{ else if eq (index .Alerts 0).Labels.resource_type "workload" }}
workload {{ (index .Alerts 0).Labels.pod_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "pod" }}
pod {{ (index .Alerts 0).Labels.pod_name }}{{ end}}is {{ (index .Alerts 0).Labels.conditionThreshold_comparison }} the threshold of {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue }}.
{{ end}}`

	Slack = `{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Target: {{ (index .Alerts 0).Labels.target_name}}
Count: {{ (index .Alerts 0).Labels.event_count}}
Event Message: {{ (index .Alerts 0).Labels.event_message}}
First Seen: {{ (index .Alerts 0).Labels.event_firstseen}}
Last Seen: {{ (index .Alerts 0).Labels.event_lastseen}}
{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}
Severity: {{ (index .Alerts 0).Labels.severity}}
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}
Resource Type: {{(index .Alerts 0).Labels.resource_type}}
Resource: {{ if eq (index .Alerts 0).Labels.resource_type "cluster" }}
cluster {{ (index .Alerts 0).Labels.cluster_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "node" }}
node {{ (index .Alerts 0).Labels.instance }}
{{ else if eq (index .Alerts 0).Labels.resource_type "workload" }}
workload {{ (index .Alerts 0).Labels.pod_name }}
{{ else if eq (index .Alerts 0).Labels.resource_type "pod" }}
pod {{ (index .Alerts 0).Labels.pod_name }}
{{ end}}
Metric: {{ (index .Alerts 0).Labels.metric}}
Comparison: {{ (index .Alerts 0).Labels.conditionThreshold_comparison}}
Threshold: {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue}}
{{ end}}
{{ if (index .Alerts 0).Labels.logs}}
Logs: {{ (index .Alerts 0).Labels.logs}}
{{ end}}`

	Email = `{{ if eq (index .Alerts 0).Labels.alert_type "event"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Target: {{ (index .Alerts 0).Labels.target_name}}<br>
Count: {{ (index .Alerts 0).Labels.event_count}}<br>
Event Message: {{ (index .Alerts 0).Labels.event_message}}<br>
First Seen: {{ (index .Alerts 0).Labels.event_firstseen}}<br>
Last Seen: {{ (index .Alerts 0).Labels.event_lastseen}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "systemService"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
{{ else if eq (index .Alerts 0).Labels.alert_type "metric"}}
Alert Name: {{ (index .Alerts 0).Labels.alert_name}}<br>
Severity: {{ (index .Alerts 0).Labels.severity}}<br>
Cluster Name: {{(index .Alerts 0).Labels.cluster_name}}<br>
Resource Type: {{(index .Alerts 0).Labels.resource_type}}<br>
Resource: {{ if eq (index .Alerts 0).Labels.resource_type "cluster" }}
cluster {{ (index .Alerts 0).Labels.cluster_name }}<br>
{{ else if eq (index .Alerts 0).Labels.resource_type "node" }}
node {{ (index .Alerts 0).Labels.instance }}<br>
{{ else if eq (index .Alerts 0).Labels.resource_type "workload" }}
workload {{ (index .Alerts 0).Labels.pod_name }}<br>
{{ else if eq (index .Alerts 0).Labels.resource_type "pod" }}
pod {{ (index .Alerts 0).Labels.pod_name }}<br>{{ end}}
Metric: {{ (index .Alerts 0).Labels.metric}}<br>
Comparison: {{ (index .Alerts 0).Labels.conditionThreshold_comparison}}<br>
Threshold: {{ (index .Alerts 0).Labels.conditionThreshold_thresholdValue}}<br>
{{ end}}
{{ if (index .Alerts 0).Labels.logs}}
Logs: {{ (index .Alerts 0).Labels.logs}}
{{ end}}
`
)
