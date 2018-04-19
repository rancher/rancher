package v3

import (
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterAlert struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterAlertSpec `json:"spec"`
	// Most recent observed status of the alert. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status AlertStatus `json:"status"`
}

type ProjectAlert struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProjectAlertSpec `json:"spec"`
	// Most recent observed status of the alert. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status AlertStatus `json:"status"`
}

type AlertCommonSpec struct {
	DisplayName           string      `json:"displayName,omitempty" norman:"required"`
	Description           string      `json:"description,omitempty"`
	Severity              string      `json:"severity,omitempty" norman:"required,options=info|critical|warning,default=critical"`
	Recipients            []Recipient `json:"recipients,omitempty" norman:"required"`
	InitialWaitSeconds    int         `json:"initialWaitSeconds,omitempty" norman:"required,default=180,min=0"`
	RepeatIntervalSeconds int         `json:"repeatIntervalSeconds,omitempty"  norman:"required,default=3600,min=0"`
}

type ClusterAlertSpec struct {
	AlertCommonSpec

	ClusterName         string               `json:"clusterName" norman:"type=reference[cluster]"`
	TargetNode          *TargetNode          `json:"targetNode,omitempty"`
	TargetSystemService *TargetSystemService `json:"targetSystemService,omitempty"`
	TargetEvent         *TargetEvent         `json:"targetEvent,omitempty"`
}

type ProjectAlertSpec struct {
	AlertCommonSpec

	ProjectName    string          `json:"projectName" norman:"type=reference[project]"`
	TargetWorkload *TargetWorkload `json:"targetWorkload,omitempty"`
	TargetPod      *TargetPod      `json:"targetPod,omitempty"`
}

type Recipient struct {
	Recipient    string `json:"recipient,omitempty"`
	NotifierName string `json:"notifierName,omitempty" norman:"required,type=reference[notifier]"`
	NotifierType string `json:"notifierType,omitempty" norman:"required,options=slack|email|pagerduty|webhook"`
}

type TargetNode struct {
	NodeName     string            `json:"nodeName,omitempty" norman:"type=reference[node]"`
	Selector     map[string]string `json:"selector,omitempty"`
	Condition    string            `json:"condition,omitempty" norman:"required,options=notready|mem|cpu,default=notready"`
	MemThreshold int               `json:"memThreshold,omitempty" norman:"min=1,max=100,default=70"`
	CPUThreshold int               `json:"cpuThreshold,omitempty" norman:"min=1,default=70"`
}

type TargetPod struct {
	PodName                string `json:"podName,omitempty" norman:"required,type=reference[/v3/projects/schemas/pod]"`
	Condition              string `json:"condition,omitempty" norman:"required,options=notrunning|notscheduled|restarts,default=notrunning"`
	RestartTimes           int    `json:"restartTimes,omitempty" norman:"min=1,default=3"`
	RestartIntervalSeconds int    `json:"restartIntervalSeconds,omitempty"  norman:"min=1,default=300"`
}

type TargetEvent struct {
	EventType    string `json:"eventType,omitempty" norman:"required,options=Normal|Warning,default=Warning"`
	ResourceKind string `json:"resourceKind,omitempty" norman:"required,options=Pod|Node|Deployment|StatefulSet|DaemonSet"`
}

type TargetWorkload struct {
	WorkloadID          string            `json:"workloadId,omitempty"`
	Selector            map[string]string `json:"selector,omitempty"`
	AvailablePercentage int               `json:"availablePercentage,omitempty" norman:"required,min=1,max=100,default=70"`
}

type TargetSystemService struct {
	Condition string `json:"condition,omitempty" norman:"required,options=etcd|controller-manager|scheduler,default=scheduler"`
}

type AlertStatus struct {
	AlertState string `json:"alertState,omitempty" norman:"options=active|inactive|alerting|muted,default=active"`
}

type Notifier struct {
	types.Namespaced

	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NotifierSpec `json:"spec"`
	// Most recent observed status of the notifier. More info:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/api-conventions.md#spec-and-status
	Status NotifierStatus `json:"status"`
}

type NotifierSpec struct {
	ClusterName string `json:"clusterName" norman:"type=reference[cluster]"`

	DisplayName     string           `json:"displayName,omitempty" norman:"required"`
	Description     string           `json:"description,omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty"`
}

type Notification struct {
	Message         string           `json:"message, omitempty"`
	SMTPConfig      *SMTPConfig      `json:"smtpConfig,omitempty"`
	SlackConfig     *SlackConfig     `json:"slackConfig,omitempty"`
	PagerdutyConfig *PagerdutyConfig `json:"pagerdutyConfig,omitempty"`
	WebhookConfig   *WebhookConfig   `json:"webhookConfig,omitempty"`
}

type SMTPConfig struct {
	Host             string `json:"host,omitempty" norman:"required,type=dnsLabel"`
	Port             int    `json:"port,omitempty" norman:"required,min=1,max=65535,default=465"`
	Username         string `json:"username,omitempty" norman:"required"`
	Password         string `json:"password,omitempty" norman:"required"`
	DefaultRecipient string `json:"defaultRecipient,omitempty" norman:"required"`
	TLS              bool   `json:"tls,omitempty" norman:"required,default=true"`
}

type SlackConfig struct {
	DefaultRecipient string `json:"defaultRecipient,omitempty" norman:"required"`
	URL              string `json:"url,omitempty" norman:"required"`
}

type PagerdutyConfig struct {
	ServiceKey string `json:"serviceKey,omitempty" norman:"required"`
}

type WebhookConfig struct {
	URL string `json:"url,omitempty" norman:"required"`
}

type NotifierStatus struct {
}

type AlertSystemImages struct {
	AlertManager       string `json:"alertManager,omitempty"`
	AlertManagerHelper string `json:"alertManagerHelper,omitempty"`
}
