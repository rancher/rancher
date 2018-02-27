package client

const (
	JobSpecType                               = "jobSpec"
	JobSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	JobSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	JobSpecFieldContainers                    = "containers"
	JobSpecFieldDNSPolicy                     = "dnsPolicy"
	JobSpecFieldFsgid                         = "fsgid"
	JobSpecFieldGids                          = "gids"
	JobSpecFieldHostAliases                   = "hostAliases"
	JobSpecFieldHostIPC                       = "hostIPC"
	JobSpecFieldHostNetwork                   = "hostNetwork"
	JobSpecFieldHostPID                       = "hostPID"
	JobSpecFieldHostname                      = "hostname"
	JobSpecFieldImagePullSecrets              = "imagePullSecrets"
	JobSpecFieldJobConfig                     = "jobConfig"
	JobSpecFieldNodeId                        = "nodeId"
	JobSpecFieldObjectMeta                    = "metadata"
	JobSpecFieldPriority                      = "priority"
	JobSpecFieldPriorityClassName             = "priorityClassName"
	JobSpecFieldRestartPolicy                 = "restartPolicy"
	JobSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	JobSpecFieldSchedulerName                 = "schedulerName"
	JobSpecFieldScheduling                    = "scheduling"
	JobSpecFieldSelector                      = "selector"
	JobSpecFieldServiceAccountName            = "serviceAccountName"
	JobSpecFieldSubdomain                     = "subdomain"
	JobSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	JobSpecFieldUid                           = "uid"
	JobSpecFieldVolumes                       = "volumes"
)

type JobSpec struct {
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
	JobConfig                     *JobConfig             `json:"jobConfig,omitempty"`
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
