package client

const (
	CronJobSpecType                               = "cronJobSpec"
	CronJobSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	CronJobSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	CronJobSpecFieldContainers                    = "containers"
	CronJobSpecFieldCronJob                       = "cronJob"
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
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	CronJob                       *CronJobConfig         `json:"cronJob,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	Fsgid                         *int64                 `json:"fsgid,omitempty"`
	Gids                          []int64                `json:"gids,omitempty"`
	HostAliases                   []HostAlias            `json:"hostAliases,omitempty"`
	HostIPC                       *bool                  `json:"hostIPC,omitempty"`
	HostNetwork                   *bool                  `json:"hostNetwork,omitempty"`
	HostPID                       *bool                  `json:"hostPID,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	ImagePullSecrets              []LocalObjectReference `json:"imagePullSecrets,omitempty"`
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
