package client

const (
	JobTemplateSpecType                               = "jobTemplateSpec"
	JobTemplateSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobTemplateSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	JobTemplateSpecFieldContainers                    = "containers"
	JobTemplateSpecFieldDNSConfig                     = "dnsConfig"
	JobTemplateSpecFieldDNSPolicy                     = "dnsPolicy"
	JobTemplateSpecFieldFsgid                         = "fsgid"
	JobTemplateSpecFieldGids                          = "gids"
	JobTemplateSpecFieldHostAliases                   = "hostAliases"
	JobTemplateSpecFieldHostIPC                       = "hostIPC"
	JobTemplateSpecFieldHostNetwork                   = "hostNetwork"
	JobTemplateSpecFieldHostPID                       = "hostPID"
	JobTemplateSpecFieldHostname                      = "hostname"
	JobTemplateSpecFieldImagePullSecrets              = "imagePullSecrets"
	JobTemplateSpecFieldJobConfig                     = "jobConfig"
	JobTemplateSpecFieldJobMetadata                   = "jobMetadata"
	JobTemplateSpecFieldNodeId                        = "nodeId"
	JobTemplateSpecFieldObjectMeta                    = "metadata"
	JobTemplateSpecFieldPriority                      = "priority"
	JobTemplateSpecFieldPriorityClassName             = "priorityClassName"
	JobTemplateSpecFieldRestartPolicy                 = "restartPolicy"
	JobTemplateSpecFieldRunAsGroup                    = "runAsGroup"
	JobTemplateSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	JobTemplateSpecFieldSchedulerName                 = "schedulerName"
	JobTemplateSpecFieldScheduling                    = "scheduling"
	JobTemplateSpecFieldSelector                      = "selector"
	JobTemplateSpecFieldServiceAccountName            = "serviceAccountName"
	JobTemplateSpecFieldShareProcessNamespace         = "shareProcessNamespace"
	JobTemplateSpecFieldSubdomain                     = "subdomain"
	JobTemplateSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	JobTemplateSpecFieldUid                           = "uid"
	JobTemplateSpecFieldVolumes                       = "volumes"
)

type JobTemplateSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	DNSConfig                     *PodDNSConfig          `json:"dnsConfig,omitempty" yaml:"dnsConfig,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty" yaml:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty" yaml:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty" yaml:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty" yaml:"hostAliases,omitempty"`
	HostIPC                       bool                   `json:"hostIPC,omitempty" yaml:"hostIPC,omitempty"`
	HostNetwork                   bool                   `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
	HostPID                       bool                   `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty" yaml:"imagePullSecrets,omitempty"`
	JobConfig                     *JobConfig             `json:"jobConfig,omitempty" yaml:"jobConfig,omitempty"`
	JobMetadata                   *ObjectMeta            `json:"jobMetadata,omitempty" yaml:"jobMetadata,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty" yaml:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsGroup                    *int64                 `json:"runAsGroup,omitempty" yaml:"runAsGroup,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty" yaml:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	ShareProcessNamespace         *bool                  `json:"shareProcessNamespace,omitempty" yaml:"shareProcessNamespace,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
