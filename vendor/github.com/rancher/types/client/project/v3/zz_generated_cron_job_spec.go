package client

const (
	CronJobSpecType                               = "cronJobSpec"
	CronJobSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	CronJobSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	CronJobSpecFieldContainers                    = "containers"
	CronJobSpecFieldCronJobConfig                 = "cronJobConfig"
	CronJobSpecFieldDNSConfig                     = "dnsConfig"
	CronJobSpecFieldDNSPolicy                     = "dnsPolicy"
	CronJobSpecFieldFsgid                         = "fsgid"
	CronJobSpecFieldGids                          = "gids"
	CronJobSpecFieldHostAliases                   = "hostAliases"
	CronJobSpecFieldHostIPC                       = "hostIPC"
	CronJobSpecFieldHostNetwork                   = "hostNetwork"
	CronJobSpecFieldHostPID                       = "hostPID"
	CronJobSpecFieldHostname                      = "hostname"
	CronJobSpecFieldImagePullSecrets              = "imagePullSecrets"
	CronJobSpecFieldNodeId                        = "nodeId"
	CronJobSpecFieldObjectMeta                    = "metadata"
	CronJobSpecFieldPriority                      = "priority"
	CronJobSpecFieldPriorityClassName             = "priorityClassName"
	CronJobSpecFieldRestartPolicy                 = "restartPolicy"
	CronJobSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	CronJobSpecFieldSchedulerName                 = "schedulerName"
	CronJobSpecFieldScheduling                    = "scheduling"
	CronJobSpecFieldSelector                      = "selector"
	CronJobSpecFieldServiceAccountName            = "serviceAccountName"
	CronJobSpecFieldSubdomain                     = "subdomain"
	CronJobSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	CronJobSpecFieldUid                           = "uid"
	CronJobSpecFieldVolumes                       = "volumes"
)

type CronJobSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty" yaml:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty" yaml:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty" yaml:"containers,omitempty"`
	CronJobConfig                 *CronJobConfig         `json:"cronJobConfig,omitempty" yaml:"cronJobConfig,omitempty"`
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
	NodeId                        string                 `json:"nodeId,omitempty" yaml:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty" yaml:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty" yaml:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty" yaml:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty" yaml:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty" yaml:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty" yaml:"scheduling,omitempty"`
	Selector                      *LabelSelector         `json:"selector,omitempty" yaml:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty" yaml:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty" yaml:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty" yaml:"volumes,omitempty"`
}
