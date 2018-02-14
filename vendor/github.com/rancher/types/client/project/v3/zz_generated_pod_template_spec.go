package client

const (
	PodTemplateSpecType                               = "podTemplateSpec"
	PodTemplateSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	PodTemplateSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	PodTemplateSpecFieldContainers                    = "containers"
	PodTemplateSpecFieldDNSPolicy                     = "dnsPolicy"
	PodTemplateSpecFieldFsgid                         = "fsgid"
	PodTemplateSpecFieldGids                          = "gids"
	PodTemplateSpecFieldHostAliases                   = "hostAliases"
	PodTemplateSpecFieldHostIPC                       = "hostIPC"
	PodTemplateSpecFieldHostNetwork                   = "hostNetwork"
	PodTemplateSpecFieldHostPID                       = "hostPID"
	PodTemplateSpecFieldHostname                      = "hostname"
	PodTemplateSpecFieldImagePullSecrets              = "imagePullSecrets"
	PodTemplateSpecFieldNodeId                        = "nodeId"
	PodTemplateSpecFieldObjectMeta                    = "metadata"
	PodTemplateSpecFieldPriority                      = "priority"
	PodTemplateSpecFieldPriorityClassName             = "priorityClassName"
	PodTemplateSpecFieldRestartPolicy                 = "restartPolicy"
	PodTemplateSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	PodTemplateSpecFieldSchedulerName                 = "schedulerName"
	PodTemplateSpecFieldScheduling                    = "scheduling"
	PodTemplateSpecFieldServiceAccountName            = "serviceAccountName"
	PodTemplateSpecFieldSubdomain                     = "subdomain"
	PodTemplateSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	PodTemplateSpecFieldUid                           = "uid"
	PodTemplateSpecFieldVolumes                       = "volumes"
)

type PodTemplateSpec struct {
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
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	RestartPolicy                 string                 `json:"restartPolicy,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
}
