package client

const (
	DaemonSetSpecType                               = "daemonSetSpec"
	DaemonSetSpecFieldActiveDeadlineSeconds         = "activeDeadlineSeconds"
	DaemonSetSpecFieldAutomountServiceAccountToken  = "automountServiceAccountToken"
	DaemonSetSpecFieldContainers                    = "containers"
	DaemonSetSpecFieldDNSPolicy                     = "dnsPolicy"
	DaemonSetSpecFieldDaemonSet                     = "daemonSet"
	DaemonSetSpecFieldFsgid                         = "fsgid"
	DaemonSetSpecFieldGids                          = "gids"
	DaemonSetSpecFieldHostAliases                   = "hostAliases"
	DaemonSetSpecFieldHostIPC                       = "hostIPC"
	DaemonSetSpecFieldHostNetwork                   = "hostNetwork"
	DaemonSetSpecFieldHostPID                       = "hostPID"
	DaemonSetSpecFieldHostname                      = "hostname"
	DaemonSetSpecFieldImagePullSecrets              = "imagePullSecrets"
	DaemonSetSpecFieldNodeId                        = "nodeId"
	DaemonSetSpecFieldObjectMeta                    = "metadata"
	DaemonSetSpecFieldPriority                      = "priority"
	DaemonSetSpecFieldPriorityClassName             = "priorityClassName"
	DaemonSetSpecFieldRestartPolicy                 = "restartPolicy"
	DaemonSetSpecFieldRunAsNonRoot                  = "runAsNonRoot"
	DaemonSetSpecFieldSchedulerName                 = "schedulerName"
	DaemonSetSpecFieldScheduling                    = "scheduling"
	DaemonSetSpecFieldSelector                      = "selector"
	DaemonSetSpecFieldServiceAccountName            = "serviceAccountName"
	DaemonSetSpecFieldSubdomain                     = "subdomain"
	DaemonSetSpecFieldTerminationGracePeriodSeconds = "terminationGracePeriodSeconds"
	DaemonSetSpecFieldUid                           = "uid"
	DaemonSetSpecFieldVolumes                       = "volumes"
)

type DaemonSetSpec struct {
	ActiveDeadlineSeconds         *int64                 `json:"activeDeadlineSeconds,omitempty"`
	AutomountServiceAccountToken  *bool                  `json:"automountServiceAccountToken,omitempty"`
	Containers                    []Container            `json:"containers,omitempty"`
	DNSPolicy                     string                 `json:"dnsPolicy,omitempty"`
	DaemonSet                     *DaemonSetConfig       `json:"daemonSet,omitempty"`
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
	Selector                      *LabelSelector         `json:"selector,omitempty"`
	ServiceAccountName            string                 `json:"serviceAccountName,omitempty"`
	Subdomain                     string                 `json:"subdomain,omitempty"`
	TerminationGracePeriodSeconds *int64                 `json:"terminationGracePeriodSeconds,omitempty"`
	Uid                           *int64                 `json:"uid,omitempty"`
	Volumes                       []Volume               `json:"volumes,omitempty"`
}
