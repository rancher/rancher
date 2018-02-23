package client

const (
	JobTemplateSpecType                               = "jobTemplateSpec"
	JobTemplateSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobTemplateSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	JobTemplateSpecFieldContainers                    = "containers"
	JobTemplateSpecFieldDNSPolicy                     = "dnsPolicy"
	JobTemplateSpecFieldFsgid                         = "fsgid"
	JobTemplateSpecFieldGids                          = "gids"
	JobTemplateSpecFieldHostAliases                   = "hostAliases"
	JobTemplateSpecFieldHostIPC                       = "hostIPC"
	JobTemplateSpecFieldHostNetwork                   = "hostNetwork"
	JobTemplateSpecFieldHostPID                       = "hostPID"
	JobTemplateSpecFieldHostname                      = "hostname"
	JobTemplateSpecFieldImagePullSecrets              = "imagePullSecrets"
	JobTemplateSpecFieldJob                           = "job"
	JobTemplateSpecFieldJobMetadata                   = "jobMetadata"
	JobTemplateSpecFieldNodeId                        = "nodeId"
	JobTemplateSpecFieldObjectMeta                    = "metadata"
	JobTemplateSpecFieldPriority                      = "priority"
	JobTemplateSpecFieldPriorityClassName             = "priorityClassName"
	JobTemplateSpecFieldRestartPolicy                 = "restartPolicy"
	JobTemplateSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	JobTemplateSpecFieldSchedulerName                 = "schedulerName"
	JobTemplateSpecFieldScheduling                    = "scheduling"
	JobTemplateSpecFieldSelector                      = "selector"
	JobTemplateSpecFieldServiceAccountName            = "serviceAccountName"
	JobTemplateSpecFieldSubdomain                     = "subdomain"
	JobTemplateSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	JobTemplateSpecFieldUid                           = "uid"
	JobTemplateSpecFieldVolumes                       = "volumes"
)

type JobTemplateSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Job                           *JobConfig             `json:"job,omitempty"`
	JobMetadata                   *ObjectMeta            `json:"jobMetadata,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
}
