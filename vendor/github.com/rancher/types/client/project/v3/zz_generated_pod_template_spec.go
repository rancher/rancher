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
	PodTemplateSpecFieldHostname                      = "hostname"
	PodTemplateSpecFieldIPC                           = "ipc"
	PodTemplateSpecFieldNet                           = "net"
	PodTemplateSpecFieldNodeId                        = "nodeId"
	PodTemplateSpecFieldObjectMeta                    = "metadata"
	PodTemplateSpecFieldPID                           = "pid"
	PodTemplateSpecFieldPriority                      = "priority"
	PodTemplateSpecFieldPriorityClassName             = "priorityClassName"
	PodTemplateSpecFieldPullPolicy                    = "pullPolicy"
	PodTemplateSpecFieldPullSecrets                   = "pullSecrets"
	PodTemplateSpecFieldRestart                       = "restart"
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
	HostAliases                   map[string]HostAlias   `json:"hostAliases,omitempty"`
	Hostname                      string                 `json:"hostname,omitempty"`
	IPC                           string                 `json:"ipc,omitempty"`
	Net                           string                 `json:"net,omitempty"`
	NodeId                        string                 `json:"nodeId,omitempty"`
	ObjectMeta                    *ObjectMeta            `json:"metadata,omitempty"`
	PID                           string                 `json:"pid,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	PullPolicy                    string                 `json:"pullPolicy,omitempty"`
	PullSecrets                   []LocalObjectReference `json:"pullSecrets,omitempty"`
	Restart                       string                 `json:"restart,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       map[string]Volume      `json:"volumes,omitempty"`
}
