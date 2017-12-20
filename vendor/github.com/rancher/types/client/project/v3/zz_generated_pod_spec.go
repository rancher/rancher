package client

const (
	PodSpecType                               = "podSpec"
	PodSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	PodSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	PodSpecFieldContainers                    = "containers"
	PodSpecFieldDNSPolicy                     = "dnsPolicy"
	PodSpecFieldFsgid                         = "fsgid"
	PodSpecFieldGids                          = "gids"
	PodSpecFieldHostAliases                   = "hostAliases"
	PodSpecFieldHostname                      = "hostname"
	PodSpecFieldIPC                           = "ipc"
	PodSpecFieldNet                           = "net"
	PodSpecFieldNodeId                        = "nodeId"
	PodSpecFieldPID                           = "pid"
	PodSpecFieldPriority                      = "priority"
	PodSpecFieldPriorityClassName             = "priorityClassName"
	PodSpecFieldPullSecrets                   = "pullSecrets"
	PodSpecFieldRestart                       = "restart"
	PodSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	PodSpecFieldSchedulerName                 = "schedulerName"
	PodSpecFieldScheduling                    = "scheduling"
	PodSpecFieldServiceAccountName            = "serviceAccountName"
	PodSpecFieldSubdomain                     = "subdomain"
	PodSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	PodSpecFieldTolerations                   = "tolerations"
	PodSpecFieldUid                           = "uid"
	PodSpecFieldVolumes                       = "volumes"
)

type PodSpec struct {
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
	PID                           string                 `json:"pid,omitempty"`
	Priority                      *int64                 `json:"priority,omitempty"`
	PriorityClassName             string                 `json:"priorityClassName,omitempty"`
	PullSecrets                   []LocalObjectReference `json:"pullSecrets,omitempty"`
	Restart                       string                 `json:"restart,omitempty"`
	RunAsNonRoot                  *bool                  `json:"runAsNonRoot,omitempty"`
	SchedulerName                 string                 `json:"schedulerName,omitempty"`
	Scheduling                    *Scheduling            `json:"scheduling,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Tolerations                   []Toleration           `json:"tolerations,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       map[string]Volume      `json:"volumes,omitempty"`
}
